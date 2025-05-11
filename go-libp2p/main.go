package main

import (
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	topicName = "pubsub"

	listenPort = 9000
)

var (
	nodeCount       = flag.Int("count", 5000, "the number of nodes in the network")
	targetPeers     = flag.Int("target", 70, "the target number of connected peers")
	gossipD         = flag.Int("d", 8, "mesh degree for gossipsub topics")
	heartbeatInterval = flag.Int("interval", 700, "heartbeat interval in milliseconds")
	messageSize     = flag.Int("size", 32, "message size in bytes")
	numMessages     = flag.Int("n", 1, "number of messages published at the same time")
)

func configureGossipParams() pubsub.GossipSubParams {
	params := pubsub.DefaultGossipSubParams()

	params.Dlo = *gossipD - 2
	params.D = *gossipD
	params.Dhi = *gossipD + 4
	params.HeartbeatInterval = time.Duration(*heartbeatInterval) * time.Millisecond
	params.HistoryLength = 6
	params.HistoryGossip = 3

	return params
}

func configurePubsubOptions() []pubsub.Option {
	options := []pubsub.Option{
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign),
		pubsub.WithNoAuthor(),
		pubsub.WithMessageIdFn(func(pmsg *pubsubpb.Message) string {
			return CalcID(pmsg.Data)
		}),
		pubsub.WithPeerOutboundQueueSize(600),
		pubsub.WithMaxMessageSize(10 * 1 << 20), 
		pubsub.WithValidateQueueSize(600),
		pubsub.WithGossipSubParams(configureGossipParams()),
	}

	return options
}

func generateNodePrivKey(nodeID int) crypto.PrivKey {

	seed := make([]byte, ed25519.SeedSize)
	binary.LittleEndian.PutUint64(seed[:8], uint64(nodeID))


	ed25519PrivateKeyData := ed25519.NewKeyFromSeed(seed)


	privKey, err := crypto.UnmarshalEd25519PrivateKey(ed25519PrivateKeyData)
	if err != nil {
		
		log.Fatalf("Failed to unmarshal private key: %v", err)
	}
	return privKey
}

func main() {
	
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	flag.Parse()


	ctx := context.Background()


	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname: %v", err)
	}

	log.Printf("Network size (count): %d\n", *nodeCount)
	log.Printf("Target connected peers: %d\n", *targetPeers)
	log.Printf("Hostname: %s\n", hostname)

	var currentNodeID int
	if _, err := fmt.Sscanf(hostname, "node%d", &currentNodeID); err != nil {
		log.Fatalf("Failed to parse node ID from hostname '%s': %v", hostname, err)
	}
	log.Printf("Node ID: %d\n", currentNodeID)

	nodePrivateKey := generateNodePrivKey(currentNodeID)

	host, err := libp2p.New(
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
		libp2p.Identity(nodePrivateKey),
	)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}

	log.Printf("Peer ID: %s\n", host.ID())
	log.Printf("Listening addresses: %v\n", host.Addrs())

	pubsubOpts := configurePubsubOptions()

	gossipSub, err := pubsub.NewGossipSub(ctx, host, pubsubOpts...)
	if err != nil {
		log.Fatalf("Failed to create GossipSub: %v", err)
	}

	topic, err := gossipSub.Join(topicName)
	if err != nil {
		log.Fatalf("Failed to join topic %s: %v", topicName, err)
	}

	subscription, err := topic.Subscribe()
	if err != nil {
		log.Fatalf("Failed to subscribe to topic %s: %v", topicName, err)
	}

	log.Println("Waiting 30 seconds for network to stabilize...")
	time.Sleep(30 * time.Second)

	log.Println("Starting peer discovery and connection...")
	connectedPeers := make(map[int]struct{})
	for len(host.Network().Peers()) < *targetPeers {
		targetNodeID := rand.Intn(*nodeCount)

		if _, isConnected := connectedPeers[targetNodeID]; isConnected || targetNodeID == currentNodeID {
			continue
		}

		targetHostname := fmt.Sprintf("node%d", targetNodeID)
		ipAddresses, err := net.LookupHost(targetHostname)
		if err != nil || len(ipAddresses) == 0 {
			log.Printf("Failed to resolve address for node%d (%s): %v. Skipping.", targetNodeID, targetHostname, err)
			continue
		}

		targetIP := ipAddresses[0]

		targetPeerID, err := peer.IDFromPrivateKey(generateNodePrivKey(targetNodeID))
		if err != nil {
			log.Printf("Failed to generate peer ID for node%d: %v. Skipping.", targetNodeID, err)
			continue
		}

		targetAddrString := fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", targetIP, listenPort, targetPeerID)
		targetAddrInfo, err := peer.AddrInfoFromString(targetAddrString)
		if err != nil {
			log.Printf("Failed to create AddrInfo for node%d (%s): %v. Skipping.", targetNodeID, targetAddrString, err)
			continue
		}

		if err = host.Connect(ctx, *targetAddrInfo); err != nil {
			log.Printf("Failed to connect to node%d (%s): %v. Retrying connection later.", targetNodeID, targetAddrString, err)
			continue
		}

		connectedPeers[targetNodeID] = struct{}{}
		log.Printf("Successfully connected to node%d: %s\n", targetNodeID, targetAddrString)
	}

	log.Printf("Achieved target of %d connected peers.", len(host.Network().Peers()))

	if currentNodeID == 0 {
		log.Printf("Node %d is the designated publisher. Publishing %d message(s)...", currentNodeID, *numMessages)
		for i := 0; i < *numMessages; i++ {
			messageData := make([]byte, *messageSize)
			if _, err := rand.Read(messageData); err != nil {
				log.Printf("Error generating random message data: %v", err)
				continue
			}

			if err := topic.Publish(ctx, messageData); err != nil {
				log.Printf("Failed to publish message by peer %s: %v", host.ID(), err)
			} else {
				log.Printf("Published message (topic: %s, id: %s)\n", topicName, CalcID(messageData))
			}
		}
		log.Println("Finished publishing messages.")
	} else {
		log.Printf("Node %d is not the designated publisher.", currentNodeID)
	}

	log.Println("Entering message reception loop...")
	for {
		receivedMessage, err := subscription.Next(ctx)
		if err != nil {
			log.Fatalf("Error receiving message from subscription: %v", err)
		}

		log.Printf("Received message (topic: %s, id: %s) from peer %s\n",
			*receivedMessage.Topic,
			CalcID(receivedMessage.Message.Data),
			receivedMessage.GetFrom(),
		)
	}

	log.Println("Node stopping.")
	// host.Close()
}

func CalcID(data []byte) string {
	if len(data) > 8 {
		return fmt.Sprintf("%x...", data[:8])
	}
	return fmt.Sprintf("%x", data)
}

