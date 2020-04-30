package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/model"

	"github.com/manifoldco/promptui"
)

const (
	createNewStr = "Create NEW Api Config"
)

type apiConfigs map[string]*model.APIConfig
type apiCfg model.APIConfig

func main() {
	ctx := context.Background()

	db, err := database.NewFromEnv(ctx)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	appPkgs, configs := readAPIConfigs(ctx, db)
	selected := getTopAction(appPkgs)

	if cfg, ok := configs[selected]; !ok {
		log.Printf("create new config not implemented yet")
	} else {
		printConfig(cfg)
	}
}

func printConfig(cfg *model.APIConfig) {
	fmt.Printf("Details for %v\n", cfg.AppPackageName)
	fmt.Printf("  AppPackageName: %v\n", cfg.AppPackageName)
	fmt.Printf(" ApkDigestSHA256: %v\n", cfg.ApkDigestSHA256)
	fmt.Printf("EnforceApkDigest: %v\n", cfg.EnforceApkDigest)
	fmt.Printf(" CTSProfileMatch: %v\n", cfg.CTSProfileMatch)
	fmt.Printf("  BasicIntegrity: %v\n", cfg.BasicIntegrity)
	fmt.Printf("   MaxAgeSeconds: %v\n", cfg.MaxAgeSeconds)
	fmt.Printf("ClockSkewSeconds: %v\n", cfg.ClockSkewSeconds)
	fmt.Printf("  AllowedRegions: %v\n", cfg.AllowedRegions)
	fmt.Printf(" AllowAllRegions: %v\n", cfg.AllowAllRegions)
	fmt.Printf(" BypassSafetynet: %v\n", cfg.BypassSafetynet)
	fmt.Printf("-----------------------\n")
}

func readAPIConfigs(ctx context.Context, db *database.DB) ([]string, apiConfigs) {
	allConfigs, err := db.ReadAPIConfigs(ctx)
	if err != nil {
		log.Fatalf("unable to list api configs: %v", err)
	}
	appPkgNames := make([]string, 0, len(allConfigs))
	configs := make(map[string]*model.APIConfig)
	for _, cfg := range allConfigs {
		configs[cfg.AppPackageName] = cfg
		appPkgNames = append(appPkgNames, cfg.AppPackageName)
	}
	return appPkgNames, configs
}

func getTopAction(configs []string) string {
	prompt := promptui.SelectWithAdd{
		Label:    "Edit Apps",
		Items:    configs,
		AddLabel: createNewStr,
	}

	_, result, err := prompt.Run()

	if err != nil {
		log.Printf("Prompt failed %v\n", err)
		return ""
	}

	return result
}
