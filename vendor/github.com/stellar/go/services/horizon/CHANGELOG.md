# Changelog

All notable changes to this project will be documented in this
file.  This project adheres to [Semantic Versioning](http://semver.org/).

As this project is pre 1.0, breaking changes may happen for minor version
bumps.  A breaking change will get clearly notified in this log.

## Unreleased

### Added

- Path payment operations now generate credit/debit effects.

### Changes

- dropped support for go1.8 since we need big.IsInt64 from math/big in our findpaths calculations

## v0.13.3 - 2018-08-23

### Bug fixes

* Fixed large amounts rendering in `/assets`.

## v0.13.2 - 2018-08-13

### Bug fixes

* Fixed a bug in `amount` and `price` packages triggering long calculations.

## v0.13.1 - 2018-07-26

### Bug fixes

* Fixed a conversion bug when `timebounds.max_time` is set to `INT64_MAX`.

## v0.13.0 - 2018-06-06

### Breaking changes

- `amount` field in `/assets` is now a String (to support Stellar amounts larger than `int64`).

### Changes

- Effect resource contains a new `created_at` field.
- Horizon responses are compressed.
- Ingestion errors have been improved.
- `horizon rebase` command was improved.

### Bug fixes

- Horizon now returns `400 Bad Request` for negative `cursor` values.

**Upgrade notes**

DB migrations add a new indexes on `history_trades`. This is very large table so migration may take a long time (depending on your DB hardware). Please test the migrations execution time on the copy of your production DB first.

## v0.12.3 - 2017-03-20

### Bug fixes

- Fix a service stutter caused by excessive `info` commands being issued from the root endpoint.


## v0.12.2 - 2017-03-14

This release is a bug fix release for v0.12.1 and v0.12.2.  *Please see the upgrade notes below if you did not already migrate your db for v0.12.0*

### Changes

- Remove strict validation on the `resolution` parameter for trade aggregations endpoint.  We will add this feature back in to the next major release. 


## v0.12.1 - 2017-03-13

This release is a bug fix release for v0.12.0.  *Please see the upgrade notes below if you did not already migrate your db for v0.12.0*

### Bug fixes

- Fixed an issue caused by un-migrated trade rows. (https://github.com/stellar/go/issues/357)
- Command line flags are now useable for subcommands of horizon.


## v0.12.0 - 2017-03-08

Big release this time for horizon:  We've made a number of breaking changes since v0.11.0 and have revised both our database schema as well as our data ingestion system.  We recommend that you take a backup of your horizon database prior to upgrading, just in case.  

### Upgrade Notes

Since this release changes both the schema and the data ingestion system, we recommend the following upgrade path to minimize downtime:

1. Upgrade horizon binaries, but do not restart the service
2. Run `horizon db migrate up` to migrate the db schema
3. Run `horizon db reingest` in a background session to begin the data reingestion process
4. Restart horizon

### Added

- Operation and payment resources were changed to add `transaction_hash` and `created_at` properties.
- The ledger resource was changed to add a `header_xdr` property.  Existing horizon installations should re-ingest all ledgers to populate the history database tables with the data.  In future versions of horizon we will disallow null values in this column.  Going forward, this change reduces the coupling of horizon to stellar-core, ensuring that horizon can re-import history even when the data is no longer stored within stellar-core's database.
- All Assets endpoint (`/assets`) that returns a list of all the assets in the system along with some stats per asset. The filters allow you to narrow down to any specific asset of interest.
- Trade Aggregations endpoint (`/trade_aggregations`) allow for efficient gathering of historical trade data. This is done by dividing a given time range into segments and aggregate statistics, for a given asset pair (`base`, `counter`) over each of these segments.

### Bug fixes

- Ingestion performance and stability has been improved. 
- Changes to an account's inflation destination no longer produce erroneous "signer_updated" effects. (https://github.com/stellar/horizon/issues/390)


### Changed

- BREAKING CHANGE: The `base_fee` property of the ledger resource has been renamed to `base_fee_in_stroops` 
- BREAKING CHANGE: The `base_reserve` property of the ledger resource has been renamed to `base_reserve_in_stroops` and is now expressed in stroops (rather than lumens) and as a JSON number. 
- BREAKING CHANGE: The "Orderbook Trades" (`/orderbook/trades`) endpoint has been removed and replaced by the "All Trades" (`/trades`) endpoint.
- BREAKING CHANGE: The Trade resource has been modified to generalize assets as (`base`, `counter`) pairs, rather than the previous (`sold`,`bought`) pairs.  
- Full reingestion (i.e. running `horizon db reingest`) now runs in reverse chronological order.  

### Removed

- BREAKING CHANGE: Friendbot has been extracted to an external microservice.


## [v0.11.0] - 2017-08-15

### Bug fixes

- The ingestion system can now properly import envelopes that contain signatures that are zero-length strings.
- BREAKING CHANGE: specifying a `limit` of `0` now triggers an error instead of interpreting the value to mean "use the default limit".
- Requests that ask for more records than the maximum page size now trigger a bad request error, instead of an internal server error.
- Upstream bug fixes to xdr decoding from `github.com/stellar/go`.

### Changed

- BREAKING CHANGE: The payments endpoint now includes `account_merge` operations in the response.
- "Finished Request" log lines now include additional fields: `streaming`, `path`, `ip`, and `host`.
- Responses now include a `Content-Disposition: inline` header.


## [v0.10.1] - 2017-03-29

### Fixed
- Ingestion was fixed to protect against text memos that contain null bytes.  While memos with null bytes are legal in stellar-core, PostgreSQL does not support such values in string columns.  Horizon now strips those null bytes to fix the issue. 

## [v0.10.0] - 2017-03-20

This is a fix release for v0.9.0 and v0.9.1


### Added
- Added `horizon db clear` helper command to clear previously ingested history.

### Fixed

- Embedded sql files for the database schema have been fixed agsain to be compatible with postgres 9.5. The configuration setting `row_security` has been removed from the dumped files.

## [v0.9.1] - 2017-03-20

### Fixed

- Embedded sql files for the database schema have been fixed to be compatible with postgres 9.5. The configuration setting `idle_in_transaction_session_timeout` has been removed from the dumped files.

## [v0.9.0] - 2017-03-20

This release was retracted due to a bug discovered after release.

### Added
- Horizon now exposes the stellar network protocol in several places:  It shows the currently reported protocol version (as returned by the stellar-core `info` command) on the root endpoint, and it reports the protocol version of each ledger resource.
- Trade resources now include a `created_at` timestamp.

### Fixed

- BREAKING CHANGE: The reingestion process has been updated.  Prior versions of horizon would enter a failed state when a gap between the imported history and the stellar-core database formed or when a previously imported ledger was no longer found in the stellar-core database.  This usually occurs when running stellar-core with the `CATCHUP_RECENT` config option.  With these changed, horizon will automatically trim "abandonded" ledgers: ledgers that are older than the core elder ledger.


## [v0.8.0] - 2017-02-07

### Added

- account signer resources now contain a type specifying the type of the signer: `ed25519_public_key`, `sha256_hash`, and `preauth_tx` are the present values used for the respective signer types.

### Changed

- The `public_key` field on signer effects and an account's signer summary has been renamed to `key` to reflect that new signer types are not necessarily specifying a public key anymore.

### Deprecated

- The `public_key` field on account signers and signer effects are deprecated

## [v0.7.1] - 2017-01-12

### Bug fixes

- Trade resources now include "bought_amount" and "sold_amount" fields when being viewed through the "Orderbook Trades" endpoint.
- Fixes #322: orderbook summaries with over 20 bids now return the correct price levels, starting with the closest to the spread.

## [v0.7.0] - 2017-01-10

### Added

- The account resource now includes links to the account's trades and data values.

### Bug fixes

- Fixes paging_token attribute of account resource
- Fixes race conditions in friendbot
- Fixes #202: Add price and price_r to "manage_offer" operation resources
- Fixes #318: order books for the native currency now filters correctly.

## [v0.6.2] - 2016-08-18

### Bug fixes

- Fixes streaming (SSE) requests, which were broken in v0.6.0

## [v0.6.1] - 2016-07-26

### Bug fixes

- Fixed an issue where accounts were not being properly returned when the  history database had no record of the account.


## [v0.6.0] - 2016-07-20

This release contains the initial implementation of the "Abridged History System".  It allows a horizon system to be operated without complete knowledge of the ledger's history.  With this release, horizon will start ingesting data from the earliest point known to the connected stellar-core instance, rather than ledger 1 as it behaved previously.  See the admin guide section titled "Ingesting stellar-core data" for more details.

### Added

- *Elder* ledgers have been introduced:  An elder ledger is the oldest ledger known to a db.  For example, the `core_elder_ledger` attribute on the root endpoint refers to the oldest known ledger stored in the connected stellar-core database.
- Added the `history-retention-count` command line flag, used to specify the amount of historical data to keep in the history db.  This is expressed as a number of ledgers, for example a value of `362880` would retain roughly 6 weeks of data given an average of 10 seconds per ledger.
- Added the `history-stale-threshold` command line flag to enable stale history protection.  See the admin guide for more info.
- Horizon now reports the last ledger ingested to stellar-core using the `setcursor` command.
- Requests for data that precede the recorded window of history stored by horizon will receive a `410 Gone` http response to allow software to differentiate from other "not found" situations.
- The new `db reap` command will manually trigger the deletion of unretained historical data
- A background process on the server now deletes unretained historical once per hour.

### Changed

- BREAKING: When making a streaming request, a normal error response will be returned if an error occurs prior to sending the first event.  Additionally, the initial http response and streaming preamble will not be sent until the first event is available.
- BREAKING: `horizon_latest_ledger` has renamed to `history_latest_ledger`
- Horizon no longer needs to begin the ingestion of historical data from ledger sequence 1.  
- Rows in the `history_accounts` table are no longer identified using the "Total Order ID" that other historical records  use, but are rather using a simple auto-incremented id.

### Removed

- The `/accounts` endpoint, which lets a consumer page through the entire set of accounts in the ledger, has been removed.  The change from complete to an abridged history in horizon makes the endpoint mostly useless, and after consulting with the community we have decided to remove the endpoint.

## [v0.5.1] - 2016-04-28

### Added

  - ManageData operation data is now rendered in the various operation end points.

### Bug fixes

- Transaction memos that contain utf-8 are now properly rendered in browsers by properly setting the charset of the http response.

## [v0.5.0] - 2016-04-22

### Added

- BREAKING: Horizon can now import data from stellar-core without the aid of the horizon-importer project.  This process is now known as "ingestion", and is enabled by either setting the `INGEST` environment variable to "true" or specifying "--ingest" on the launch arguments for the horizon process.  Only one process should be running in this mode for any given horizon database.
- Add `horizon db init`, used to install the latest bundled schema for the horizon database.
- Add `horizon db reingest` command, used to update outdated or corrupt horizon database information.  Admins may now use `horizon db reingest outdated` to migrate any old data when updated horizon.
- Added `network_passphrase` field to root resource.
- Added `fee_meta_xdr` field to transaction resource.

### Bug fixes
- Corrected casing on the "offers" link of an account resource.

## [v0.4.0] - 2016-02-19

### Added

- Add `horizon db migrate [up|down|redo]` commands, used for installing schema migrations.  This work is in service of porting the horizon-importer project directly to horizon.
- Add support for TLS: specify `--tls-cert` and `--tls-key` to enable.
- Add support for HTTP/2.  To enable, use TLS.

### Removed

- BREAKING CHANGE: Removed support for building on go versions lower than 1.6

## [v0.3.0] - 2016-01-29

### Changes

- Fixed incorrect `source_amount` attribute on pathfinding responses.
- BREAKING CHANGE: Sequence numbers are now encoded as strings in JSON responses.
- Fixed broken link in the successful response to a posted transaction

## [v0.2.0] - 2015-12-01
### Changes

- BREAKING CHANGE: the `address` field of a signer in the account resource has been renamed to `public_key`.
- BREAKING CHANGE: the `address` on the account resource has been renamed to `account_id`.

## [v0.1.1] - 2015-12-01

### Added
- Github releases are created from tagged travis builds automatically

[v0.11.0]: https://github.com/stellar/horizon/compare/v0.10.1...v0.11.0
[v0.10.1]: https://github.com/stellar/horizon/compare/v0.10.0...v0.10.1
[v0.10.0]: https://github.com/stellar/horizon/compare/v0.9.1...v0.10.0
[v0.9.1]: https://github.com/stellar/horizon/compare/v0.9.0...v0.9.1
[v0.9.0]: https://github.com/stellar/horizon/compare/v0.8.0...v0.9.0
[v0.8.0]: https://github.com/stellar/horizon/compare/v0.7.1...v0.8.0
[v0.7.1]: https://github.com/stellar/horizon/compare/v0.7.0...v0.7.1
[v0.7.0]: https://github.com/stellar/horizon/compare/v0.6.2...v0.7.0
[v0.6.2]: https://github.com/stellar/horizon/compare/v0.6.1...v0.6.2
[v0.6.1]: https://github.com/stellar/horizon/compare/v0.6.0...v0.6.1
[v0.6.0]: https://github.com/stellar/horizon/compare/v0.5.1...v0.6.0
[v0.5.1]: https://github.com/stellar/horizon/compare/v0.5.0...v0.5.1
[v0.5.0]: https://github.com/stellar/horizon/compare/v0.4.0...v0.5.0
[v0.4.0]: https://github.com/stellar/horizon/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/stellar/horizon/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/stellar/horizon/compare/v0.1.1...v0.2.0
[v0.1.1]: https://github.com/stellar/horizon/compare/v0.1.0...v0.1.1
