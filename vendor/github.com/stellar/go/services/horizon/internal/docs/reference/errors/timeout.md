---
title: Timeout
---

If you are encountering this error it means that either:

* Horizon has not received a confirmation from the Core server that the transaction you are trying to submit to the network was included in a ledger in a timely manner or:
* Horizon has not sent a response to a reverse-proxy before in a specified time.

The former case may happen because there was no room for your transaction in the 3 consecutive ledgers. In such case, Core server removes a transaction from a queue. To solve this you can:

* Keep resubmitting the same transaction (with the same sequence number) and wait until it finally is added to a new ledger or:
* Increase the [fee](/developers/guides/concepts/fees.html).

## Attributes

As with all errors Horizon returns, `timeout` follows the [Problem Details for HTTP APIs](https://tools.ietf.org/html/draft-ietf-appsawg-http-problem-00) draft specification guide and thus has the following attributes:

| Attribute | Type   | Description                                                                                                                     |
| --------- | ----   | ------------------------------------------------------------------------------------------------------------------------------- |
| Type      | URL    | The identifier for the error.  This is a URL that can be visited in the browser.                                                |
| Title     | String | A short title describing the error.                                                                                             |
| Status    | Number | An HTTP status code that maps to the error.                                                                                     |
| Detail    | String | A more detailed description of the error.                                                                                       |
| Instance  | String | A token that uniquely identifies this request. Allows server administrators to correlate a client report with server log files  |


## Related

[Not Acceptable](./not-acceptable.md)
[Transaction Failed](./transaction-failed.md)
