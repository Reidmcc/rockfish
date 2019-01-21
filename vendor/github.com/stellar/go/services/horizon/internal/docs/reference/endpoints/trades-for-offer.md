---
title: Trades for Offer
clientData:
  laboratoryUrl: https://www.stellar.org/laboratory/#explorer?resource=trades&endpoint=for_offer
---

This endpoint represents all [trades](../resources/trade.md) for a given [offer](../resources/offer.md).

## Request

```
GET /offers/{offer_id}/trades{?cursor,limit,order}
```

### Arguments

| name | notes | description | example |
| ---- | ----- | ----------- | ------- |
| `offer_id` | required, number | ID of an offer | 323223 |
| `?cursor` | optional, any, default _null_ | A paging token, specifying where to start returning records from. | 12884905984 |
| `?order`  | optional, string, default `asc` | The order in which to return rows, "asc" or "desc". | `asc` |
| `?limit`  | optional, number, default: `10` | Maximum number of records to return. | `200` |

### curl Example Request

```sh
curl "https://horizon-testnet.stellar.org/offers/323223/trades"
```

### JavaScript Example Request

```js
var StellarSdk = require('stellar-sdk');
var server = new StellarSdk.Server('https://horizon-testnet.stellar.org');

server.trades()
    .forOffer(323223)
    .call()
    .then(function (tradesResult) {
      console.log(tradesResult);
    })
    .catch(function (err) {
      console.error(err);
    })
```


## Response

This endpoint responds with a list of trades that consumed a given offer. See the [trade resource](../resources/trade.md) for reference.

### Example Response
```json
{
  "_links": {
    "self": {
      "href": "https://horizon-testnet.stellar.org/offers/323223/trades?cursor=\u0026limit=10\u0026order=asc"
    },
    "next": {
      "href": "https://horizon-testnet.stellar.org/offers/323223/trades?cursor=35789107779080193-0\u0026limit=10\u0026order=asc"
    },
    "prev": {
      "href": "https://horizon-testnet.stellar.org/offers/323223/trades?cursor=35789107779080193-0\u0026limit=10\u0026order=desc"
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
            "href": "https://horizon-testnet.stellar.org/accounts/GDRCFIQAUEFUQ6GXF5DPRO2M77E4UB7RW7EWI2FTKOW7CWYKZCHSI75K"
          },
          "counter": {
            "href": "https://horizon-testnet.stellar.org/accounts/GCUD7CBKTQI4D7ZR7IKHMGXZKKVABML7XFBHV4AIYBOEN5UQFZ5DSPPT"
          },
          "operation": {
            "href": "https://horizon-testnet.stellar.org/operations/35789107779080193"
          }
        },
        "id": "35789107779080193-0",
        "paging_token": "35789107779080193-0",
        "ledger_close_time": "2018-04-08T05:58:37Z",
        "offer_id": "323223",
        "base_account": "GDRCFIQAUEFUQ6GXF5DPRO2M77E4UB7RW7EWI2FTKOW7CWYKZCHSI75K",
        "base_amount": "912.6607285",
        "base_asset_type": "native",
        "counter_account": "GCUD7CBKTQI4D7ZR7IKHMGXZKKVABML7XFBHV4AIYBOEN5UQFZ5DSPPT",
        "counter_amount": "16.5200719",
        "counter_asset_type": "credit_alphanum4",
        "counter_asset_code": "CM10",
        "counter_asset_issuer": "GBUJJAYHS64L4RDHPLURQJUKSHHPINSAYXYVMWPEF4LECHDKB2EFMKBX",
        "base_is_seller": true,
        "price": {
          "n": 18101,
          "d": 1000000
        }
      }
    ]
  }
}
```

## Possible Errors

- The [standard errors](../errors.md#Standard-Errors).
- [not_found](../errors/not-found.md): A `not_found` error will be returned if there is no offer whose ID matches the `offer_id` argument.
