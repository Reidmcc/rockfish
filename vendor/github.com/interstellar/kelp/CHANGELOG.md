# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Deprecated

### Removed

### Fixed

### Security

## [v1.3.0] - 2019-01-10

### Added
- mirror strategy offsets trades onto the backing exchange, run `glide up` to udpate dependencies ([3a703a359db541b636cab38c3dd8a7fbe6df7193](https://github.com/interstellar/kelp/commit/3a703a359db541b636cab38c3dd8a7fbe6df7193))
- ccxt integration now supports trading APIs for all exchanges ([5cf0aedc67eff89a8f82082326f878844ac7b5d5](https://github.com/interstellar/kelp/commit/5cf0aedc67eff89a8f82082326f878844ac7b5d5))
- randomized delay via the MAX_TICK_DELAY_MILLIS ([4b74affb9933bf08a093ee66cea46c1b3fb87753](https://github.com/interstellar/kelp/commit/4b74affb9933bf08a093ee66cea46c1b3fb87753))

### Changed
- balanced strategy avoids unncessary re-randomization on every update cycle ([0be414c77c2f12c9b4b624922aea5841e84c704c](https://github.com/interstellar/kelp/commit/0be414c77c2f12c9b4b624922aea5841e84c704c))

### Fixed
- fix op_underfunded issue when hitting capacity limits ([d339e421f82de9e2996e45e71d745d81dff2f3f0](https://github.com/interstellar/kelp/commit/d339e421f82de9e2996e45e71d745d81dff2f3f0))

## [v1.2.0] - 2018-11-26

### Added
- support for alerting with PagerDuty as the first implementation, run `glide up` to update the dependency ([5e46ae0d94751d85dbb2e8f73094f5d96af0df5e](https://github.com/interstellar/kelp/commit/5e46ae0d94751d85dbb2e8f73094f5d96af0df5e))
- support for logging to a file with the `--log` or `-l` command-line option followed by the prefix of the log filename
- support for basic monitoring with a health check service, run `glide up` to update the dependency ([c6374c35cff9dfa46da342aa5342f312dcd337c4](https://github.com/interstellar/kelp/commit/c6374c35cff9dfa46da342aa5342f312dcd337c4))
- `iter` command line param to run for only a fixed number of iterations, run `glide up` to update the dependencies
- new DELETE_CYCLES_THRESHOLD config value in trader config file to allow some tolerance of errors before deleting all offers ([f2537cafee8d620e1c4aabdd3d072d90628801b8](https://github.com/interstellar/kelp/commit/f2537cafee8d620e1c4aabdd3d072d90628801b8))

### Changed
- reduced the number of available assets that are recognized by the GetOpenOrders() API for Kraken
- levels are now logged with prices in the quote asset and amounts in the base asset for the sell, buysell, and balanced strategies
- clock tick is now synchronized at the start of each cycle ([cd33d91b2d468bfbce6d38a6186d12c86777b7d5](https://github.com/interstellar/kelp/commit/cd33d91b2d468bfbce6d38a6186d12c86777b7d5))

### Fixed
- conversion of asset symbols in the GetOpenOrders() API for Kraken, reducing the number of tested asset symbols with this API
- fix op_underfunded errors when we hit capacity limits for non-XLM assets ([e6bebee9aeadf6e00a829a28c125f5dffad8c05c](https://github.com/interstellar/kelp/commit/e6bebee9aeadf6e00a829a28c125f5dffad8c05c))

## [v1.1.2] - 2018-10-30

### Added
- log balance with liabilities

### Changed
- scripts/build.sh: update VERSION format and LDFLAGS to include git branch info

### Fixed
- fix op_underfunded errors when we hit capacity limits

## [v1.1.1] - 2018-10-22

### Fixed
- fixed bot panicing when it cannot cast ticker bid/ask values to a float64 from CCXT's FetchTicker endpoint (0ccbc495e18b1e3b207dad5d3421c7556c63c004) ([issue #31](https://github.com/interstellar/kelp/issues/31))

## [v1.1.0] - 2018-10-19

### Added
- support for [CCXT](https://github.com/ccxt/ccxt) via [CCXT-REST API](https://github.com/franz-see/ccxt-rest), increasing exchange integrations for priceFeeds and mirroring [diff](https://github.com/interstellar/kelp/compare/0db8f2d42580aa87867470e428d5f0f63eed5ec6^...33bc7b98418129011b151d0f56c9c0770a3d897e)

## [v1.0.0] - 2018-10-15

### Changed
- executables for windows should use the .exe extension (7b5bbc9eb5b776a27c63483c4af09ca38937670d)

### Fixed
- fixed divide by zero error (fa7d7c4d5a2a256d6cfcfe43a65e530e3c06862e)

## [v1.0.0-rc3] - 2018-09-29

### Added
- support for all currencies available on Kraken

## [v1.0.0-rc2] - 2018-09-28

### Added
- This CHANGELOG file

### Changed
- Updated dependency `github.com/stellar/go` to latest version `5bbd27814a3ffca9aeffcbd75a09a6164959776a`, run `glide up` to update this dependency

### Fixed
- If `SOURCE_SECRET_SEED` is missing or empty then the bot will not crash now.
- support for [CAP-0003](https://github.com/stellar/stellar-protocol/blob/master/core/cap-0003.md) introduced in stellar-core protocol v10 ([issue #2](https://github.com/interstellar/kelp/issues/2))

## v1.0.0-rc1 - 2018-08-13

### Added
- Kelp bot with a few basic strategies, priceFeeds, and support for integrating with the Kraken Exchange.
- Modular design allowing anyone to plug in their own strategies
- Robust logging
- Configuration file based approach to setting up a bot
- Documentation on existing capabilities

[Unreleased]: https://github.com/interstellar/kelp/compare/v1.3.0...HEAD
[v1.3.0]: https://github.com/interstellar/kelp/compare/v1.2.0...v1.3.0
[v1.2.0]: https://github.com/interstellar/kelp/compare/v1.1.2...v1.2.0
[v1.1.2]: https://github.com/interstellar/kelp/compare/v1.1.1...v1.1.2
[v1.1.1]: https://github.com/interstellar/kelp/compare/v1.1.0...v1.1.1
[v1.1.0]: https://github.com/interstellar/kelp/compare/v1.0.0...v1.1.0
[v1.0.0]: https://github.com/interstellar/kelp/compare/v1.0.0-rc3...v1.0.0
[v1.0.0-rc3]: https://github.com/interstellar/kelp/compare/v1.0.0-rc2...v1.0.0-rc3
[v1.0.0-rc2]: https://github.com/interstellar/kelp/compare/v1.0.0-rc1...v1.0.0-rc2
