package modules

import (
	"fmt"

	"github.com/interstellar/kelp/support/utils"
	"github.com/stellar/go/build"
	"github.com/stellar/go/clients/horizon"
	"github.com/stellar/go/support/config"
)

type arbitCycleConfig struct {
	HoldAssetCode   string       `valid:"-" toml:"HOLD_ASSET_CODE"`
	HoldAssetIssuer string       `valid:"-" toml:"HOLD_ASSET_ISSUER"`
	Assets          []assetInput `valid:"-" toml:"ASSETS"`
}

// String impl.
func (c arbitCycleConfig) String() string {
	return utils.StructString(c, nil)
}

// assetInput holds the inbound asset strings
type assetInput struct {
	CODE   string `valid:"-"`
	ISSUER string `valid:"-"`
}

// PathManager keeps track of all the possible payment paths
type PathManager struct {
	multiDex  MultiDex
	assetBook []*horizon.Asset
	pathList  []PaymentPath
}

// PathList is a set of payment paths
// type PathList struct {
// 	paths []PaymentPath
// }

// PaymentPath is a pair of assets for a payment path
type PaymentPath struct {
	pathAssetA *horizon.Asset
	pathAssetB *horizon.Asset
}

// MakePathManager is the factory method
func MakePathManager(
	multiDex MultiDex,
	stratConfigPath string,
	simMode bool,
) (*PathManager, error) {
	if stratConfigPath == "" {
		return nil, fmt.Errorf("arbitcycle mode needs a config file")
	}

	var cfg arbitCycleConfig
	err := config.Read(stratConfigPath, &cfg)
	utils.CheckConfigError(cfg, err, stratConfigPath)
	utils.LogConfig(cfg)

	var assetBook []*horizon.Asset

	for _, a := range cfg.Assets {
		asset, e := ParseAsset(a.CODE, a.ISSUER)
		if e != nil {
			return nil, fmt.Errorf("Error while generating assetBook %s", e)
		}
		assetBook = append(assetBook, asset)
	}

	var pathList []PaymentPath

	for i := 0; i < len(assetBook); i++ {
		for n := 0; n < len(assetBook); n++ {
			if assetBook[i] != assetBook[n] {
				path := PaymentPath{
					assetBook[i],
					assetBook[n],
				}
				pathList = append(pathList, path)
			}
		}
	}
	return &PathManager{
		multiDex:  multiDex,
		assetBook: assetBook,
		pathList:  pathList,
	}, nil
}

// ParseAsset returns a horizon asset from strings
// This is temporary until the sdex feed is merged into Kelp master
// Actually I can't do this mode at all without the sdex feeds, so either wait or copy them
// copy them for local testing, eh?
func ParseAsset(code string, issuer string) (*horizon.Asset, error) {
	if code != "XLM" && issuer == "" {
		return nil, fmt.Errorf("error: issuer can only be empty if asset is XLM")
	}

	if code == "XLM" && issuer != "" {
		return nil, fmt.Errorf("error: issuer needs to be empty if asset is XLM")
	}

	if code == "XLM" {
		asset := utils.Asset2Asset2(build.NativeAsset())
		return &asset, nil
	}

	asset := utils.Asset2Asset2(build.CreditAsset(code, issuer))
	return &asset, nil
}
