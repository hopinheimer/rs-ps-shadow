use std::error::Error;
use tracing::info;
use chrono::Local;
use std::time::{Duration, Instant};
use clap::Parser;
use futures::StreamExt;
use libp2p::{core, gossipsub, identity, noise, yamux, SwarmBuilder, Transport, PeerId, Multiaddr};
use libp2p::gossipsub::MessageAuthenticity;
use std::net::ToSocketAddrs;
use libp2p::swarm::{NetworkBehaviour, SwarmEvent};
use tokio::select;
use rand::rngs::{OsRng,StdRng};
use rand::{Rng, SeedableRng};
use sha2::{Digest, Sha256};
use tracing_subscriber::{prelude::*, registry::Registry, fmt, EnvFilter};
use std::hash::{Hash, Hasher};
use openssl::rand::rand_bytes;
use std::collections::hash_map::DefaultHasher;
use tracing::debug_span;
mod writer;


#[derive(Parser,Debug)]
struct Opts {

    #[arg(long, default_value = "10")]
    count: u64,

    #[arg(long, default_value = "8")]
    target: usize,

    #[arg(long, default_value = "8")]
    d: usize,

    #[arg(long, default_value = "700")]
    interval: u64,

    #[arg(long, default_value = "32")]
    size: usize,

    #[arg(long, default_value = "1")]
    n: usize,
}

#[derive(NetworkBehaviour)]
struct MyBehaviour{
    gossipsub: gossipsub::Behaviour
}


fn deterministic_keypair(id: u64) -> identity::Keypair {

    let mut seed = [0u8; 32];
    seed[..8].copy_from_slice(&id.to_le_bytes());
    identity::Keypair::ed25519_from_bytes(&mut seed).expect("infallible")

}

fn get_node_id_from_hostname() -> u64 {
    let hostname = hostname::get().unwrap().to_str().unwrap().to_string();
    let node_id_str = hostname.trim_start_matches("node");
    node_id_str.parse::<u64>().expect("Hostname must be in format node{id}")
}

fn generate_seed_from_id(id: &str) -> u64 {
    let mut hasher = DefaultHasher::new();
    id.hash(&mut hasher);
    hasher.finish()
}

fn generate_random_message_with_seed(id: &str) -> [u8; 128] {
    let seed = generate_seed_from_id(id);
    let mut rng = StdRng::seed_from_u64(seed);
    let mut msg = [0u8; 128];
    rng.fill(&mut msg);
    msg
}

#[tokio::main]
async fn main()-> Result<(), Box<dyn Error>>{
    info!("simulation start time {}", Local::now().to_rfc3339());
    let opts = Opts::parse();

    let node_id = get_node_id_from_hostname();
    info!("Starting node with id: {}", node_id);

    let id_keys = deterministic_keypair(node_id);
    let local_peer_id = PeerId::from(id_keys.public());
    info!("Local peer id: {:?}", local_peer_id);

    let node_span = debug_span!("libp2p_gossipsub::behaviour", node_id  = node_id);
    let _entered = node_span.enter(); 


    let subscriber = Registry::default()
        .with(fmt::Layer::default()
        .json()
        .with_current_span(true))
        .with(EnvFilter::new("debug"));

    tracing::subscriber::set_global_default(subscriber).expect("failed to set global subscriber");


    let private_key = identity::Keypair::generate_ed25519();
    let yamux_config = yamux::Config::default();

    let tcp = libp2p::tcp::tokio::Transport::new(libp2p::tcp::Config::default().nodelay(true))
        .upgrade(core::upgrade::Version::V1)
        .authenticate(noise::Config::new(&private_key)
            .expect("infallible"))
        .multiplex(yamux_config)
        .timeout(Duration::from_secs(30));


    let mut gossipsub = {
        let message_id_fn = |message: &gossipsub::Message| {
            use sha2::{Sha256, Digest};
            let hash = Sha256::digest(&message.data);
            gossipsub::MessageId::from(hash.as_slice())
};

        let gossipsub_config = gossipsub::ConfigBuilder::default() 
            .mesh_n(opts.d)
            .mesh_n_low(opts.d - 2)
            .message_id_fn(message_id_fn)
            .mesh_n_high(opts.d + 4)
            .max_transmit_size(10*1024*1024)
            .heartbeat_interval(Duration::from_millis(opts.interval))
            .validation_mode(gossipsub::ValidationMode::Anonymous)
            .build()
            .expect("infallible");


        gossipsub::Behaviour::new(MessageAuthenticity::Anonymous, gossipsub_config).expect("yos?")
    };

    let pubsub_topic = "pubsub";


    let topic = gossipsub::IdentTopic::new(pubsub_topic);
    gossipsub.subscribe(&topic).unwrap();

    let mut swarm = {
        let builder = SwarmBuilder::with_existing_identity(private_key)
            .with_tokio()
            .with_other_transport(|_key| tcp.boxed())
            .expect("infallible");

        builder
            .with_behaviour(|_| MyBehaviour{ gossipsub })
            .expect("infallible")
            .build()
    };

    swarm.listen_on("/ip4/0.0.0.0/tcp/9000".parse()?)?;

    let mut connected = 0;
    let mut tried = std::collections::HashSet::new();

    while connected < opts.target {
        let peer_id_guess = rand::random::<u64>() % opts.count;
        if peer_id_guess == node_id || tried.contains(&peer_id_guess) {
            continue;
        }
        tried.insert(peer_id_guess);

        let hostname = format!("node{}", peer_id_guess);
        if let Ok(addrs) = (hostname.as_str(), 9000).to_socket_addrs() {
            for addr in addrs {
                let ip = addr.ip();
                let multiaddr: Multiaddr = format!("/ip4/{}/tcp/9000", ip)
                    .parse().unwrap();

                info!("Dialing {:?}", multiaddr);
                match swarm.dial(multiaddr) {
                    Ok(_) => {
                        connected += 1;
                        info!("Connected to node{}", peer_id_guess);
                        break;
                    }
                    Err(_) => {

                        // no-ops
                    }
                }
            }
        }
    }

    info!("discovery complete");

    let mut published = true;
    let mut last_checked = Instant::now();
    let mut rng = rand::thread_rng();

    loop {
        select!{
            event = swarm.select_next_some() => {
                match event {
                    SwarmEvent::NewListenAddr{address, ..} => {
                        info!("listening on {:?}", address);
                    }
                    SwarmEvent::Behaviour(MyBehaviourEvent::Gossipsub(gossip_event)) => {parse_gossip(gossip_event);}
                    _ => { info!("random event: {:?}", event)}
                }
            }
        }

        let now = Instant::now();

        if now.duration_since(last_checked)>= Duration::from_millis(150){
            last_checked = now;

            if rng.gen_range(1..=3) == 1 {
                published = true;
            }
        }

        if node_id==0 &&  published && swarm.connected_peers().count()>=opts.target {

            info!("trying to publish message");
            
            let mut msg = [0u8;128];
            rand_bytes(&mut msg).unwrap();
            rng.fill(&mut msg[..]);

            match swarm
                .behaviour_mut()
                .gossipsub
                .publish(topic.clone(), msg.clone()) {
                Ok(message) => {
                    info!(
                        "published msg (topic: {}, id: {}, message: {})",
                        topic,
                        hex::encode(Sha256::digest(&msg)),
                        message
                    );
                }
                Err(e) => {
                    info!("Number of peers in topic {}: {}", topic, swarm.connected_peers().count());

                    info!("failed to publish message: {:?}", e);
                }
            }
            info!("published");
            published = false;
        }
    }
}

fn parse_gossip(event: gossipsub::Event){

    match event {
        gossipsub::Event::Message { propagation_source, message_id, message } => { info!("event {:?}", message)}
        gossipsub::Event::Subscribed { peer_id, topic } => { info!("subscribed: topic={:?}", topic)}
        gossipsub::Event::Unsubscribed { peer_id, topic } => { info!("event {:?}", topic)}
        gossipsub::Event::SlowPeer { peer_id, failed_messages } => { info!("event {:?}", peer_id)}
        gossipsub::Event::GossipsubNotSupported { peer_id } => { info!("event {:?}", peer_id)}

    }
}
