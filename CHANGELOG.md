# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## Unreleased
### Changed
- Modified payment amount determination to reduce by 2% from the maximum available in the DEX path; this is make it less likely that out-of-app rounding code cause payments to become too expensive.


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
