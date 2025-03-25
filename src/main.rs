use std::error::Error;
use clap::Parser;
use futures::StreamExt;
use libp2p::{core, gossipsub, identity, noise, yamux, SwarmBuilder, Transport};
use libp2p::gossipsub::MessageAuthenticity;
use libp2p::swarm::{NetworkBehaviour, SwarmEvent};
use tokio::select;

#[derive(Parser,Debug)]
struct Opts {

    #[arg(long, default_value = "5000")]
    count: usize,

    #[arg(long, default_value = "70")]
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

#[tokio::main]
async fn main()-> Result<(), Box<dyn Error>>{

    let opts = Opts::parse();

    let private_key = identity::Keypair::generate_ed25519();
    let yamux_config = yamux::Config::default();

    let tcp = libp2p::tcp::tokio::Transport::new(libp2p::tcp::Config::default().nodelay(true))
        .upgrade(core::upgrade::Version::V1)
        .authenticate(noise::Config::new(&private_key)
            .expect("infallible"))
        .multiplex(yamux_config)
        .timeout(std::time::Duration::from_secs(20));

    let gossipsub = {
        let gossipsub_config = gossipsub::ConfigBuilder::default()
            .validation_mode(gossipsub::ValidationMode::Anonymous)
            .build()
            .expect("infallible");

        gossipsub::Behaviour::new(MessageAuthenticity::Anonymous, gossipsub_config).expect("yos?")
    };

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

    loop{
        select!{
            event = swarm.select_next_some() => {
                match event{
                    SwarmEvent::NewListenAddr{address, ..} => {
                        println!("Listening on {:?}", address);
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
                    _ =>{}
                }
            }
        }
    }
}
