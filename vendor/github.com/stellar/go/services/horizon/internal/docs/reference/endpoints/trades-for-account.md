---
title: Trades for Account
clientData:
  laboratoryUrl: https://www.stellar.org/laboratory/#explorer?resource=trades&endpoint=for_account
---

This endpoint represents all [trades](../resources/trade.md) that affect a given [account](../resources/account.md).

## Request

```
GET /accounts/{account_id}/trades{?cursor,limit,order}
```

### Arguments

| name | notes | description | example |
| ---- | ----- | ----------- | ------- |
| `account_id` | required, string | ID of an account | GBYTR4MC5JAX4ALGUBJD7EIKZVM7CUGWKXIUJMRSMK573XH2O7VAK3SR |
| `?cursor` | optional, any, default _null_ | A paging token, specifying where to start returning records from. When streaming this can be set to `now` to stream object created since your request time. | 12884905984 |
| `?order`  | optional, string, default `asc` | The order in which to return rows, "asc" or "desc". | `asc` |
| `?limit`  | optional, number, default: `10` | Maximum number of records to return. | `200` |

### curl Example Request

```sh
curl "https://horizon-testnet.stellar.org/accounts/GBYTR4MC5JAX4ALGUBJD7EIKZVM7CUGWKXIUJMRSMK573XH2O7VAK3SR/trades?limit=1"
```

### JavaScript Example Request

```js
var StellarSdk = require('stellar-sdk');
var server = new StellarSdk.Server('https://horizon-testnet.stellar.org');

server.trades()
  .forAccount("GCEZWKCA5VLDNRLN3RPRJMRZOX3Z6G5CHCGSNFHEYVXM3XOJMDS674JZ")
  .call()
  .then(function (accountResult) {
    console.log(accountResult);
  })
  .catch(function (err) {
    console.error(err);
  })
```


## Response

This endpoint responds with a list of trades that changed a given account's state. See the [trade resource](../resources/trade.md) for reference.

### Example Response
```json
{
  "_links": {
    "self": {
      "href": "/accounts/GBYTR4MC5JAX4ALGUBJD7EIKZVM7CUGWKXIUJMRSMK573XH2O7VAK3SR/trades?cursor=\u0026limit=1\u0026order=asc"
    },
    "next": {
      "href": "/accounts/GBYTR4MC5JAX4ALGUBJD7EIKZVM7CUGWKXIUJMRSMK573XH2O7VAK3SR/trades?cursor=940258535411713-0\u0026limit=1\u0026order=asc"
    },
    "prev": {
      "href": "/accounts/GBYTR4MC5JAX4ALGUBJD7EIKZVM7CUGWKXIUJMRSMK573XH2O7VAK3SR/trades?cursor=940258535411713-0\u0026limit=1\u0026order=desc"
    }
  },
  "_embedded": {
    "records": [
      {
        "_links": {
          "self": {
            "href": ""
          },
          "base": {
            "href": "/accounts/GBYTR4MC5JAX4ALGUBJD7EIKZVM7CUGWKXIUJMRSMK573XH2O7VAK3SR"
          },
          "counter": {
            "href": "/accounts/GBOOAYCAJIN7YCUUAHEQJJARNQMRUP4P2WXVO6P4KAMAB27NGA3CYTZU"
          },
          "operation": {
            "href": "/operations/940258535411713"
          }
        },
        "id": "940258535411713-0",
        "paging_token": "940258535411713-0",
        "ledger_close_time": "2017-03-30T13:20:41Z",
        "offer_id": "8",
        "base_account": "GBYTR4MC5JAX4ALGUBJD7EIKZVM7CUGWKXIUJMRSMK573XH2O7VAK3SR",
        "base_amount": "1.0000000",
        "base_asset_type": "credit_alphanum4",
        "base_asset_code": "BTC",
        "base_asset_issuer": "GB6FN4C7ZLWKENAOZDLZOQHNIOK4RDMV6EKLR53LWCHEBR6LVXOEKDZH",
        "counter_account": "GBOOAYCAJIN7YCUUAHEQJJARNQMRUP4P2WXVO6P4KAMAB27NGA3CYTZU",
        "counter_amount": "1.0000000",
        "counter_asset_type": "native",
        "base_is_seller": true,
        "price": {
          "n": 1,
          "d": 1
        }
      }
    ]
  }
}
```

## Possible Errors

- The [standard errors](../errors.md#Standard-Errors).
- [not_found](../errors/not-found.md): A `not_found` error will be returned if there is no account whose ID matches the `account_id` argument.
