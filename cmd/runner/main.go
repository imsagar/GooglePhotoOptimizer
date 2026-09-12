// Command runner is the local GPOptimizer agent: pairs with the server and
// connects over WebSocket to transcode videos.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/user/gpoptimizer/runner/config"
	"github.com/user/gpoptimizer/runner/ffmpeg"
	"github.com/user/gpoptimizer/runner/pairing"
	"github.com/user/gpoptimizer/runner/ws"
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

	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Not paired yet. Run with: --pair <CODE> --server <URL>")
		os.Exit(1)
	}

	if _, err := ffmpeg.EnsureInstalled(); err != nil {
		log.Fatal(err)
	}

	// cmdFn is nil until Task 10 wires in the transcode pipeline; until then
	// incoming commands are just logged (see ws.Client.handleCommand).
	ws.Run(cfg, nil)
}
