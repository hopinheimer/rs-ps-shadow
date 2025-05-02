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
	gossipD         = flag.Int("D", 8, "mesh degree for gossipsub topics")
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

// generateNodePrivKey computes a private key for a given node ID.
// This uses a deterministic approach based on the node ID to ensure consistent keys.
func generateNodePrivKey(nodeID int) crypto.PrivKey {
	// Create a seed from the node ID.
	seed := make([]byte, ed25519.SeedSize)
	binary.LittleEndian.PutUint64(seed[:8], uint64(nodeID))

	// Generate an Ed25519 private key from the seed.
	ed25519PrivateKeyData := ed25519.NewKeyFromSeed(seed)

	// Unmarshal the Ed25519 private key into a libp2p crypto.PrivKey.
	privKey, err := crypto.UnmarshalEd25519PrivateKey(ed25519PrivateKeyData)
	if err != nil {
		// This should ideally not happen if the seed is valid.
		log.Fatalf("Failed to unmarshal private key: %v", err)
	}
	return privKey
}

func main() {
	// Configure logging output and flags.
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// Parse command-line flags.
	flag.Parse()

	// Create a background context.
	ctx := context.Background()

	// Get the hostname of the current machine.
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Failed to get hostname: %v", err)
	}

	log.Printf("Network size (count): %d\n", *nodeCount)
	log.Printf("Target connected peers: %d\n", *targetPeers)
	log.Printf("Hostname: %s\n", hostname)

	// Extract the node ID from the hostname (assuming hostname is in the format "nodeX").
	var currentNodeID int
	// Using Fscanf to parse the node ID from the hostname string.
	if _, err := fmt.Sscanf(hostname, "node%d", &currentNodeID); err != nil {
		log.Fatalf("Failed to parse node ID from hostname '%s': %v", hostname, err)
	}
	log.Printf("Node ID: %d\n", currentNodeID)

	// Generate the private key for the current node.
	nodePrivateKey := generateNodePrivKey(currentNodeID)

	// Create a new libp2p host.
	host, err := libp2p.New(
		// Listen on all IP4 addresses on the specified port.
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
		// Set the node's identity using the generated private key.
		libp2p.Identity(nodePrivateKey),
	)
	if err != nil {
		log.Fatalf("Failed to create libp2p host: %v", err)
	}

	log.Printf("Peer ID: %s\n", host.ID())
	log.Printf("Listening addresses: %v\n", host.Addrs())

	// Configure pubsub options.
	pubsubOpts := configurePubsubOptions()

	// Create a new GossipSub router.
	gossipSub, err := pubsub.NewGossipSub(ctx, host, pubsubOpts...)
	if err != nil {
		log.Fatalf("Failed to create GossipSub: %v", err)
	}

	// Join the specified pubsub topic.
	topic, err := gossipSub.Join(topicName)
	if err != nil {
		log.Fatalf("Failed to join topic %s: %v", topicName, err)
	}

	// Subscribe to the topic to receive messages.
	subscription, err := topic.Subscribe()
	if err != nil {
		log.Fatalf("Failed to subscribe to topic %s: %v", topicName, err)
	}

	// Wait for a period to allow other nodes to start and bootstrap.
	log.Println("Waiting 30 seconds for network to stabilize...")
	time.Sleep(30 * time.Second)

	log.Println("Starting peer discovery and connection...")
	// Discover and connect to target number of peers.
	connectedPeers := make(map[int]struct{})
	// Continue connecting until the target number of peers is reached.
	for len(host.Network().Peers()) < *targetPeers {
		// Randomly select a node ID to potentially connect to.
		targetNodeID := rand.Intn(*nodeCount)

		// Skip if the target node is the current node or already connected.
		if _, isConnected := connectedPeers[targetNodeID]; isConnected || targetNodeID == currentNodeID {
			continue
		}

		// Resolve the IP addresses of the target node's hostname.
		targetHostname := fmt.Sprintf("node%d", targetNodeID)
		ipAddresses, err := net.LookupHost(targetHostname)
		if err != nil || len(ipAddresses) == 0 {
			log.Printf("Failed to resolve address for node%d (%s): %v. Skipping.", targetNodeID, targetHostname, err)
			continue
		}

		// Get the first IP address found.
		targetIP := ipAddresses[0]

		// Generate the peer ID for the target node.
		targetPeerID, err := peer.IDFromPrivateKey(generateNodePrivKey(targetNodeID))
		if err != nil {
			log.Printf("Failed to generate peer ID for node%d: %v. Skipping.", targetNodeID, err)
			continue
		}

		// Craft a multiaddress for the target peer.
		targetAddrString := fmt.Sprintf("/ip4/%s/tcp/%d/p2p/%s", targetIP, listenPort, targetPeerID)
		targetAddrInfo, err := peer.AddrInfoFromString(targetAddrString)
		if err != nil {
			log.Printf("Failed to create AddrInfo for node%d (%s): %v. Skipping.", targetNodeID, targetAddrString, err)
			continue
		}

		// Connect to the target peer.
		if err = host.Connect(ctx, *targetAddrInfo); err != nil {
			log.Printf("Failed to connect to node%d (%s): %v. Retrying connection later.", targetNodeID, targetAddrString, err)
			// Optionally, add a short delay before the next connection attempt.
			// time.Sleep(100 * time.Millisecond)
			continue
		}

		// Mark the peer as connected.
		connectedPeers[targetNodeID] = struct{}{}
		log.Printf("Successfully connected to node%d: %s\n", targetNodeID, targetAddrString)
	}

	log.Printf("Achieved target of %d connected peers.", len(host.Network().Peers()))

	if currentNodeID == 0 {
		log.Printf("Node %d is the designated publisher. Publishing %d message(s)...", currentNodeID, *numMessages)
		for i := 0; i < *numMessages; i++ {
			// Create a message of the specified size with random data.
			messageData := make([]byte, *messageSize)
			// rand.Read populates the slice with random bytes.
			// It's generally fast enough for this purpose.
			if _, err := rand.Read(messageData); err != nil {
				log.Printf("Error generating random message data: %v", err)
				continue // Skip this message if data generation fails.
			}

			// Publish the message to the topic.
			if err := topic.Publish(ctx, messageData); err != nil {
				log.Printf("Failed to publish message by peer %s: %v", host.ID(), err)
			} else {
				// Log the publication with the computed message ID.
				// Assuming CalcID is defined elsewhere.
				log.Printf("Published message (topic: %s, id: %s)\n", topicName, CalcID(messageData))
			}
		}
		log.Println("Finished publishing messages.")
	} else {
		log.Printf("Node %d is not the designated publisher.", currentNodeID)
	}

	// Main loop to receive and process messages.
	log.Println("Entering message reception loop...")
	for {
		// Block and wait for the next message from the subscription.
		receivedMessage, err := subscription.Next(ctx)
		if err != nil {
			// If there's an error receiving, log it and exit the loop.
			log.Fatalf("Error receiving message from subscription: %v", err)
		}

		// Log the received message details.
		// Assuming CalcID is defined elsewhere.
		log.Printf("Received message (topic: %s, id: %s) from peer %s\n",
			*receivedMessage.Topic,
			CalcID(receivedMessage.Message.Data),
			receivedMessage.GetFrom(), // GetFrom returns the peer ID of the message sender.
		)
	}

	// The code will only reach here if the reception loop is exited (due to a fatal error).
	log.Println("Node stopping.")
	// In a real application, you might want to gracefully close the host here.
	// host.Close()
}

// Placeholder for CalcID function. Replace with the actual implementation.
// This function is assumed to exist in the original code and is required for message ID calculation.
func CalcID(data []byte) string {
	// Example placeholder: return a hash or a simple string representation.
	// A common approach is to hash the message data.
	// For this example, returning a simple string representation of the first few bytes.
	if len(data) > 8 {
		return fmt.Sprintf("%x...", data[:8])
	}
	return fmt.Sprintf("%x", data)
}

