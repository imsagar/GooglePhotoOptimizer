// Command runner is the local GPOptimizer agent: pairs with the server and
// (once Task 8 lands) connects over WebSocket to transcode videos.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/user/gpoptimizer/runner/config"
	"github.com/user/gpoptimizer/runner/pairing"
)

func main() {
	pairCode := flag.String("pair", "", "Pairing code from the web UI")
	serverURL := flag.String("server", "https://gpoptimizer.example.com", "Server URL")
	flag.Parse()

	if *pairCode != "" {
		if _, err := pairing.Pair(*serverURL, *pairCode); err != nil {
			log.Fatalf("Pairing failed: %v", err)
		}
		fmt.Println("Paired successfully! Runner will now connect automatically.")
		fmt.Printf("Config saved to %s\n", config.Path())
		return
	}

	if _, err := config.Load(); err != nil {
		fmt.Println("Not paired yet. Run with: --pair <CODE> --server <URL>")
		os.Exit(1)
	}

	// TODO(task 8): ensure FFmpeg and connect over WebSocket.
	fmt.Println("Runner not yet implemented. Run with --pair to set up.")
}
