![rockfish banner](https://user-images.githubusercontent.com/43561569/52517026-2be8b480-2bfa-11e9-9f95-4379a7010ad1.png)


Rockfish is an arbitrage bot for the Stellar Decentralized Exchange (SDEX). The SDEX is native to [Stellar's](https://www.stellar.org/) blockchain, and you can interact with it through third-party frontends, such as [Stellar X](https://www.stellarx.com/) and [Stellarport](https://stellarport.io/home).

Due to the specifics of the SDEX, the way Rockfish works is quite different from general arbitrage. First, Rockfish uses a same-exchange cross-asset strategy, instead of buying an asset on one exchange and selling the same asset on another.

More importantly, Rockfish doesn't technically perform trades; it makes payments. One of Stellar's headline features is [atomic multi-currency transactions](https://www.stellar.org/how-it-works/stellar-basics/explainers/#Multi-currency_transactions). These payments route assets through the SDEX, using available buy and sell orders. It's essentially a currency exchange service. It is also the equivalent of buying an asset, using that asset to buy a second asset, and selling the second asset back into the destination asset. If the orders line up favorably and you set the destination asset to the start asset, it's possible to make a profit. The underlying trades all execute together, so the intermediate assets are never held by either the payer or the recepient, greatly alleviating the risk that sequential trades would incur.

All of which also adds up to rationalizing price discovery on the SDEX. Pretty great!

### Using Rockfish

Check out the [walkthrough](https://github.com/Reidmcc/rockfish/blob/master/walkthrough.md). Please note Rockfish is in _**very**_ early alpha and under heavy development; proceed with care.

### Installing Rockfish

Either grab one of the releases, or you can compile from source, see below. Terminal commands in these instructions are mostly for Linux.

1. Clone this repository
2. Install the [Go programming language](https://golang.org/)
3. Install [Glide](https://github.com/Masterminds/glide) `curl https://glide.sh/get | sh`
4. Run `glide install` and `glide up` (for Windows too)
5. Run Rockfish's `build.sh` from the main Rockfish repo directory `./scripts/build.sh`
6. You should now have a `bin` folder in your repository with an executable: `rockfish`

### Acknowledgments

Rockfish is largely built from [Kelp](https://github.com/interstellar/kelp) components and would not be possible without them. Real rockfish live in [kelp forests](https://en.wikipedia.org/wiki/Kelp_forest); hence the name.

#### Disclaimer

Nothing in Rockfish or its documentation should be taken as investment advice. Rockfish is available as-is, on the terms of the [MIT License](https://github.com/Reidmcc/rockfish/blob/master/LICENSE).
