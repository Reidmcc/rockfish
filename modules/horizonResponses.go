package modules

// FindPathResponse holds the raw JSON for a horizon call
type FindPathResponse struct {
	Embedded struct {
		Records []PathRecord `json:"records"`
	} `json:"_embedded"`
}

// PathRecord is a single horizon find path record
type PathRecord struct {
	SourceAssetType        string      `json:"source_asset_type"`
	SourceAmount           string      `json:"source_amount"`
	DestinationAssetType   string      `json:"destination_asset_type"`
	DestinationAssetCode   string      `json:"destination_asset_code"`
	DestinationAssetIssuer string      `json:"destination_asset_issuer"`
	DestinationAmount      string      `json:"destination_amount"`
	Path                   []PathAsset `json:"path"`
}

// PathAsset is a single asset in a path
type PathAsset struct {
	AssetType   string `json:"asset_type"`
	AssetCode   string `json:"asset_code"`
	AssetIssuer string `json:"asset_issuer"`
}
