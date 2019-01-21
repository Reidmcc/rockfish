---
title: Account
---

In the Stellar network, users interact using **accounts** which can be controlled by a corresponding keypair that can authorize transactions. One can create a new account with the [Create Account](./operation.md#create-account) operation.

To learn more about the concept of accounts in the Stellar network, take a look at the [Stellar account concept guide](https://www.stellar.org/developers/learn/concepts/accounts.html).

When horizon returns information about an account it uses the following format:

## Attributes
| Attribute    | Type             |                                                                                                                        |
|--------------|------------------|------------------------------------------------------------------------------------------------------------------------|
| id           | string           | The canonical id of this account, suitable for use as the :id parameter for url templates that require an account's ID. |
| account_id      | string           | The account's public key encoded into a base32 string representation.                                                    |
| sequence     | number           | The current sequence number that can be used when submitting a transaction from this account.                           |
| subentry_count     | number           | The number of [account subentries](https://www.stellar.org/developers/guides/concepts/ledger.html#ledger-entries). |
| balances     | array of objects | An array of the native asset or credits this account holds.                                                          |
| thresholds     | object | An object of account flags. |
| flags                    | array of objects | The flags denote the enabling/disabling of certain asset issuer privileges. |
| signers     | array of objects | An array of account signers with their weights. |
| data     | object | An array of account [data](./data.md) fields. |

#### Signer Object

| Attribute    | Type             |                                                                                                                        |
|--------------|------------------|------------------------------------------------------------------------------------------------------------------------|
| public_key           | string           | **Deprecated:** replaced by `key`. |
| weight      | number           | The numerical weight of a signer, necessary to determine whether a transaction meets the threshold requirements.                                                   |
| key     | string           | Different depending on the type of the signer.                            |
| type     | string           | See below. |

#### Possible Signer Types
| Type         |                                                                                                                        |
|--------------|------------------------------------------------------------------------------------------------------------------------|
| ed25519_public_key | A normal Stellar public key. |
| sha256_hash        | The SHA256 hash of some arbitrary `x`. Adding a signature of this type allows anyone who knows `x` to sign a transaction from this account. *Note: once this transaction is broadcast, `x` will be known publicly.*  |
| preauth_tx         | The hash of a pre-authorized transaction. This signer is automatically removed from the account when a matching transaction is properly applied. |


#### Balances Object
| Attribute    | Type             |                                                                                                                        |
|--------------|------------------|------------------------------------------------------------------------------------------------------------------------|
| balance           | string           | How much of an asset is owned. |
| buying_liabilities    | string           | The total amount of an asset offered to buy aggregated over all offers owned by this account. |
| selling_liabilities    | string           | The total amount of an asset offered to sell aggregated over all offers owned by this account. |
| limit      | optional, number           |  The maximum amount of an asset that this account is willing to accept (this is specified when an account opens a trustline).                                           |
| asset_type    | string           | Either native, credit_alphanum4, or credit_alphanum12.                        |
| asset_code     | optional, string           | The code for the asset.                       |
| asset_issuer     | optional, string           | The stellar address of the given asset's issuer.  |

#### Flag Object
|    Attribute     |  Type  |                                                                                                                                |
| ---------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------ |
| auth_immutable             | bool | With this setting, none of the following authorization flags can be changed. |
| auth_required              | bool | With this setting, an anchor must approve anyone who wants to hold its asset.  |
| auth_revocable             | bool | With this setting, an anchor can set the authorize flag of an existing trustline to freeze the assets held by an asset holder.  |

#### Threshold Object
| Attribute    | Type             |                                                                                                                        |
|--------------|------------------|------------------------------------------------------------------------------------------------------------------------|
| low_threshold           | number           | The weight required for a valid transaction including the [Allow Trust](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#allow-trust) and [Bump Sequence](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#bump-sequence) operations.|
| med_threshold      | number           | The weight required for a valid transaction including the [Create Account](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#create-account), [Payment](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#payment), [Path Payment](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#path-payment), [Manage Offer](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#manage-offer), [Create Passive Offer](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#create-passive-offer), [Change Trust](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#change-trust), [Inflation](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#inflation), and [Manage Data](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#manage-data) operations.                                               |
| high_threshold     | number           | The weight required for a valid transaction including the [Account Merge](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#account-merge) and [Set Options](https://www.stellar.org/developers/guides/concepts/list-of-operations.html#set-options) operations.                |

## Links
| rel          | Example                                                                                           | Description                                                | `templated` |
|--------------|---------------------------------------------------------------------------------------------------|------------------------------------------------------------|-------------|
| data      | `/accounts/GAOEWNUEKXKNGB2AAOX6S6FEP6QKCFTU7KJH647XTXQXTMOAUATX2VF5/data/{key}`      | [Data fields](./data.md) related to this account           | true        |
| effects      | `/accounts/GAOEWNUEKXKNGB2AAOX6S6FEP6QKCFTU7KJH647XTXQXTMOAUATX2VF5/effects/{?cursor,limit,order}`      | The [effects](./effect.md) related to this account           | true        |
| offers       | `/accounts/GAOEWNUEKXKNGB2AAOX6S6FEP6QKCFTU7KJH647XTXQXTMOAUATX2VF5/offers/{?cursor,limit,order}`       | The [offers](./offer.md) related to this account             | true        |
| operations   | `/accounts/GAOEWNUEKXKNGB2AAOX6S6FEP6QKCFTU7KJH647XTXQXTMOAUATX2VF5/operations/{?cursor,limit,order}`   | The [operations](./operation.md) related to this account     | true        |
| payments   | `/accounts/GAOEWNUEKXKNGB2AAOX6S6FEP6QKCFTU7KJH647XTXQXTMOAUATX2VF5/payments/{?cursor,limit,order}`   | The [payments](./payment.md) related to this account     | true        |
| trades | `/accounts/GAOEWNUEKXKNGB2AAOX6S6FEP6QKCFTU7KJH647XTXQXTMOAUATX2VF5/trades/{?cursor,limit,order}` | The [trades](./trade.md) related to this account | true        |
| transactions | `/accounts/GAOEWNUEKXKNGB2AAOX6S6FEP6QKCFTU7KJH647XTXQXTMOAUATX2VF5/transactions/{?cursor,limit,order}` | The [transactions](./transaction.md) related to this account | true        |


## Example

```json
{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23"
    },
    "transactions": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23/transactions{?cursor,limit,order}",
      "templated": true
    },
    "operations": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23/operations{?cursor,limit,order}",
      "templated": true
    },
    "payments": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23/payments{?cursor,limit,order}",
      "templated": true
    },
    "effects": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23/effects{?cursor,limit,order}",
      "templated": true
    },
    "offers": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23/offers{?cursor,limit,order}",
      "templated": true
    },
    "trades": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23/trades{?cursor,limit,order}",
      "templated": true
    },
    "data": {
      "href": "https://horizon-testnet.stellar.org/accounts/GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23/data/{key}",
      "templated": true
    }
  },
  "id": "GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23",
  "paging_token": "",
  "account_id": "GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23",
  "sequence": "26509955490119684",
  "subentry_count": 1,
  "thresholds": {
    "low_threshold": 0,
    "med_threshold": 0,
    "high_threshold": 0
  },
  "flags": {
    "auth_required": false,
    "auth_revocable": false
  },
  "balances": [
    {
      "balance": "9999.9999600",
      "asset_type": "native"
    }
  ],
  "signers": [
    {
      "public_key": "GBRTWTVW65NO4AER7W6G5CTVWGZCLQJIKJTAX523Q5GPU6TNJONXOR23",
      "weight": 1
    }
  ],
  "data": {
    "club": "MTAw"
  }
}
```

## Endpoints

| Resource                 | Type       | Resource URI Template                |
|--------------------------|------------|--------------------------------------|
| [Account Details](../endpoints/accounts-single.md)      | Single     | `/accounts/:id`                      |
| [Account Data](../endpoints/data-for-account.md)      | Single     | `/accounts/:id/data/:key`                      |
| [Account Transactions](../endpoints/transactions-for-account.md) | Collection | `/accounts/:account_id/transactions` |
| [Account Operations](../endpoints/operations-for-account.md)   | Collection | `/accounts/:account_id/operations`   |
| [Account Payments](../endpoints/payments-for-account.md)     | Collection | `/accounts/:account_id/payments`     |
| [Account Effects](../endpoints/effects-for-account.md)      | Collection | `/accounts/:account_id/effects`      |
| [Account Offers](../endpoints/offers-for-account.md)       | Collection | `/accounts/:account_id/offers`       |
