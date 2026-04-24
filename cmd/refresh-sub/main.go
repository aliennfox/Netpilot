// refresh-sub: 拉取所有订阅、重建 data/merged.json、重启 sing-box。
// 供 scripts/start-singbox.sh 在启动前调用，避免 merged.json 过期。
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/foxnetpilot/netpilot/internal/config"
	"github.com/foxnetpilot/netpilot/internal/engine"
	"github.com/foxnetpilot/netpilot/internal/overlay"
	"github.com/foxnetpilot/netpilot/internal/subscription"
)

func main() {
	cfg := config.Default()

	baseAbs, err := filepath.Abs("configs/minimal.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve base config: %v\n", err)
		os.Exit(1)
	}

	ov := overlay.NewConfigOverlay(baseAbs, cfg.DataDir)
	if err := ov.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "load overlay: %v\n", err)
		os.Exit(1)
	}

	adapter := engine.NewSingBoxAdapter(cfg.ClashAPIAddr)
	mergedAbs, _ := filepath.Abs(ov.MergedConfigPath())
	adapter.SetConfigPath(mergedAbs)

	store := subscription.NewSubscriptionStore(cfg.DataDir)
	if err := store.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "load subscription store: %v\n", err)
		os.Exit(1)
	}
	if len(store.List()) == 0 {
		fmt.Println("no subscriptions saved; skipping refresh")
		return
	}

	mgr := subscription.NewSubscriptionManager(store, ov, adapter)
	fmt.Println(mgr.UpdateAll())
}
