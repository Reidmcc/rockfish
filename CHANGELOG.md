# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## Unreleased
### Added
- Implemented the ability to use different hold assets for different asset groups. **This change required a restructuring of the arbitcycle.cfg parameters, so configuration files from version 0.3.3-alpha or earlier will not work with later versions**. See the [example file](examples/sample_arbitconfig.cfg) for the new setup.
- Added a rate-limiter option to space out multi-threaded API calls that in some cases could trigger the Horizon API rate limit. The new option is in arbitconfig.cfg, as `RATE_LIMITER_MS`. **A line for this option must be present in the configuration file, even if it is set to 0.**

### Changed
- Changed the conceptulization of asset groups. 
 - arbitconfig.cfg now has an additional level of nesting, `[[GROUPS]]`, with the `[[ASSETS]]` lists now named `[[GROUPS.ASSETS]]`
 - Hold assets are now considered part of the asset group and must be specified inside each `[[GROUPS]]` entry, including if the hold asset is the same across groups

### Fixed
- Fixed an issue that caused balances for hold assets other than XLM to always return as unfindable.

### Removed
- Removed the deprecated pathRequestor code

## v 0.3.3-alpha
### Fixed
- Fixed a nil reference that could occur when a `calculatePathValues` call returned an error
### Removed
- `TICK_INTERVAL_SECONDS` is no longer supported; users will need to remove it from their `arbitconfig.cfg` file

## v 0.3.2-alpha

### Fixed
- Fixed a nil reference that occured when no valid route was found

## v 0.3.1-alpha
### Fixed
- Fixed a cpu leak from unclosed tickers in stream routines

## v 0.3.0-alpha

### Added
- Implemented multithreading throughout, greatly improving speed
- Added streamingfrom Horizon orderbooks to synchronize Rockfish with Stellar's ledger updates

### Changed
- SDEX orderbooks are now queried from Horizon once and distributed to the relevant paths, instead of calling the orderbooks during path calculations
- The log now displays "route empty" in the place of a ratio when the path is broken by an empty orderbook, instead of displaying "0"
- Modified payment amount determination to reduce by 2% from the maximum available in the DEX path; this is make it less likely that out-of-app rounding code cause payments to become too expensive
- Due to multithreading the log will not always show the list of route results in the same order

### Deprecated
- `TICK_INTERVAL_SECONDS` is no longer used; replaced by stream synchronization
- The `--iter` flag no longer has an effect due to deprecation of `TICK_INTERVAL_SECONDS`



## v 0.2.1-alpha
### Fixed
- Fixed amount calculation problems stemming from not inverting bid amounts


## v 0.2.0-alpha
### Added
- Added minimum trade parameter to prevent losses on very small trades due to fees
### Changed
- Changed from floating-point math to model.Number math throughout
- Changed first trading pair to sell viewpoint to simplify ratio calculation
### Fixed
- Fixed amount calculations failing to convert back to hold asset values


## v 0.1.0-alpha
### Added
- Core Rockfish functionality: watch orderbooks, find opportunities, and make cyclical path payments

### Public API definition
- For the purposes of Semantic Versioning, the public API for Rockfish is the configuration file structure

![rockfish icon long flip](https://user-images.githubusercontent.com/43561569/52517024-0c518c00-2bfa-11e9-9cd0-e2443d7868f1.png)
