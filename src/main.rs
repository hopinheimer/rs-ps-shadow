use std::error::Error;
use chrono::Local;
use std::time::Duration;
use clap::Parser;
use futures::StreamExt;
use libp2p::{core, gossipsub, identity, noise, yamux, SwarmBuilder, Transport, PeerId, Multiaddr};
use libp2p::gossipsub::MessageAuthenticity;
use std::net::ToSocketAddrs;
use libp2p::swarm::{NetworkBehaviour, SwarmEvent};
use tokio::{select , time::sleep};
use rand::Rng;
use sha2::{Digest, Sha256};


#[derive(Parser,Debug)]
struct Opts {

    #[arg(long, default_value = "10")]
    count: u64,

    #[arg(long, default_value = "8")]
    target: u64,

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

fn reconstruct_peer_id(id: u64) -> PeerId {
    PeerId::from(deterministic_keypair(id).public())
}

fn get_node_id_from_hostname() -> u64 {
    let hostname = hostname::get().unwrap().to_str().unwrap().to_string();
    let node_id_str = hostname.trim_start_matches("node");
    node_id_str.parse::<u64>().expect("Hostname must be in format node{id}")
}

#[tokio::main]
async fn main()-> Result<(), Box<dyn Error>>{
    println!("simulation start time {}", Local::now().to_rfc3339());
    let opts = Opts::parse();

    let node_id = get_node_id_from_hostname();
    println!("Starting node with id: {}", node_id);

    let id_keys = deterministic_keypair(node_id);
    let local_peer_id = PeerId::from(id_keys.public());
    println!("Local peer id: {:?}", local_peer_id);

    

    let private_key = identity::Keypair::generate_ed25519();
    let yamux_config = yamux::Config::default();

    let tcp = libp2p::tcp::tokio::Transport::new(libp2p::tcp::Config::default().nodelay(true))
        .upgrade(core::upgrade::Version::V1)
        .authenticate(noise::Config::new(&private_key)
            .expect("infallible"))
        .multiplex(yamux_config)
        .timeout(Duration::from_secs(30));


    let mut gossipsub = {
        let gossipsub_config = gossipsub::ConfigBuilder::default()
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

                println!("Dialing {:?}", multiaddr);
                match swarm.dial(multiaddr) {
                    Ok(_) => {
                        connected += 1;
                        println!("Connected to node{}", peer_id_guess);
                        break;
                    }
                    Err(_) => {

                        // no-ops
                    }
                }
            }
        }
    }

    println!("discovery complete");

//    sleep(Duration::from_secs(45)).await;


    if node_id == 0 {

        println!("trying to publish message");
        for _ in 0..opts.n {
            let mut msg = vec![0u8; opts.size];
            rand::thread_rng().fill(&mut msg[..]);

            match swarm.behaviour_mut().gossipsub.publish(topic.clone(), msg.clone()) {
                Ok(_) => {
                    println!(
                        "published msg (topic: {}, id: {})",
                        topic,
                        hex::encode(Sha256::digest(&msg))
                    );
                }
                Err(e) => {
                    eprintln!("failed to publish message: {:?}", e);
                }
            }
        }
    }
     
    loop{
        select!{
            event = swarm.select_next_some() => {
                match event{
                    SwarmEvent::NewListenAddr{address, ..} => {
                        println!("listening on {:?}", address);
                    }
                    SwarmEvent::Behaviour(MyBehaviourEvent::Gossipsub(gossipsub::Event::Message {
                        propagation_source: peer_id,
                        message_id: id,
                        message,
                    })) => {
                        println!(
                            "Got message: {} with id: {} from peer: {:?}",
                            String::from_utf8_lossy(&message.data),
                            id,
                            peer_id
                        )
                    }
                    _ =>{
                        println!("event: {:?}",event);
                    }
                }
            }
        }
    }
}
