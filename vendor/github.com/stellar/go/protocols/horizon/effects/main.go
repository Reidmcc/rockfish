package effects

import (
	"time"
	"github.com/stellar/go/protocols/horizon/base"
	"github.com/stellar/go/support/render/hal"
)


// Base provides the common structure for any effect resource effect.
type Base struct {
	Links struct {
		Operation hal.Link `json:"operation"`
		Succeeds  hal.Link `json:"succeeds"`
		Precedes  hal.Link `json:"precedes"`
	} `json:"_links"`

	ID              string    `json:"id"`
	PT              string    `json:"paging_token"`
	Account         string    `json:"account"`
	Type            string    `json:"type"`
	TypeI           int32     `json:"type_i"`
	LedgerCloseTime time.Time `json:"created_at"`
}

// PagingToken implements `hal.Pageable`
func (this Base) PagingToken() string {
	return this.PT
}

type AccountCreated struct {
	Base
	StartingBalance string `json:"starting_balance"`
}

type AccountCredited struct {
	Base
	base.Asset
	Amount string `json:"amount"`
}

type AccountDebited struct {
	Base
	base.Asset
	Amount string `json:"amount"`
}

type AccountThresholdsUpdated struct {
	Base
	LowThreshold  int32 `json:"low_threshold"`
	MedThreshold  int32 `json:"med_threshold"`
	HighThreshold int32 `json:"high_threshold"`
}

type AccountHomeDomainUpdated struct {
	Base
	HomeDomain string `json:"home_domain"`
}

type AccountFlagsUpdated struct {
	Base
	AuthRequired  *bool `json:"auth_required_flag,omitempty"`
	AuthRevokable *bool `json:"auth_revokable_flag,omitempty"`
}

type SequenceBumped struct {
	Base
	NewSeq int64 `json:"new_seq"`
}

type SignerCreated struct {
	Base
	Weight    int32  `json:"weight"`
	PublicKey string `json:"public_key"`
	Key       string `json:"key"`
}

type SignerRemoved struct {
	Base
	Weight    int32  `json:"weight"`
	PublicKey string `json:"public_key"`
	Key       string `json:"key"`
}

type SignerUpdated struct {
	Base
	Weight    int32  `json:"weight"`
	PublicKey string `json:"public_key"`
	Key       string `json:"key"`
}

type TrustlineCreated struct {
	Base
	base.Asset
	Limit string `json:"limit"`
}

type TrustlineRemoved struct {
	Base
	base.Asset
	Limit string `json:"limit"`
}

type TrustlineUpdated struct {
	Base
	base.Asset
	Limit string `json:"limit"`
}

type TrustlineAuthorized struct {
	Base
	Trustor   string `json:"trustor"`
	AssetType string `json:"asset_type"`
	AssetCode string `json:"asset_code,omitempty"`
}

type TrustlineDeauthorized struct {
	Base
	Trustor   string `json:"trustor"`
	AssetType string `json:"asset_type"`
	AssetCode string `json:"asset_code,omitempty"`
}

type Trade struct {
	Base
	Seller            string `json:"seller"`
	OfferID           int64  `json:"offer_id"`
	SoldAmount        string `json:"sold_amount"`
	SoldAssetType     string `json:"sold_asset_type"`
	SoldAssetCode     string `json:"sold_asset_code,omitempty"`
	SoldAssetIssuer   string `json:"sold_asset_issuer,omitempty"`
	BoughtAmount      string `json:"bought_amount"`
	BoughtAssetType   string `json:"bought_asset_type"`
	BoughtAssetCode   string `json:"bought_asset_code,omitempty"`
	BoughtAssetIssuer string `json:"bought_asset_issuer,omitempty"`
}

// interface implementations
var _ base.Rehydratable = &SignerCreated{}
var _ base.Rehydratable = &SignerRemoved{}
var _ base.Rehydratable = &SignerUpdated{}
