### Using Rockfish for Cross-asset Arbitrage

Rockfish's strategy is to take advantage of market inefficiencies by selling through multiple trading pairs where there is price-mismatch. Using the assets you configure, Rockfish finds paths through which it can pass value and end up with more than it started with. 

Rockfish needs a "hold asset"; the asset you want to actually have and acquire more of. On the SDEX, the native lumen (XLM) is the obvious candidate as it has a wide range of trading pairs, but you can use any asset that trades against several others.

You also need at least two other assets through witch to pass your trades. The situation that Rockfish uses is where it can buy one of the non-hold assets, sell it for the other non-hold asset, and then sell that asset back into the hold asset. Using US dollars (USD) and Euros (EUR) as an example, the path could be:

XLM :arrow_right: USD :arrow_right: EUR :arrow_right: XLM

or

XLM :arrow_right: EUR :arrow_right: USD :arrow_right: XLM

### Preparation

1. Get Rockfish, either by downloading a release or compiling from source (see the README for compiling instructions).

2. Create at least one Stellar network address. There are [several wallets](https://www.stellar.org/lumens/wallets/) available, and the Stellar foundation has a [detailed guide](https://www.stellar.org/developers/guides/get-started/create-account.html).

3. Aquire assets. There are Stellar network on-ramps for major cryptocurrencies and some fiat currencies, or you could buy lumens on a crypto exchange. Some of the web wallets providers also provide on-ramp functionality. 

### Set up your configuration files. Example files are [here](https://github.com/Reidmcc/rockfish/tree/master/examples). 

You will need both a base configuration file for your account and network information and a configuration file for Rockfish's settings.

#### Arbitconfig

The base configuration file is fairly straightforward, with these items:

The secret seed for the account Rockfish will use for its activity: 

`TRADING_SECRET_SEED="[THESEEDFORYOURACCOUNTDONTPOSTITONGITHUB]"`

-----------------

For trading strategies that use actual trade offers you can use a separate source account for fees and sequence numbers, as Kelp does. For now, Rockfish does not do that, so leave this one as `""`:

`SOURCE_SECRET_SEED=""`

-----------------

How often you want Rockfish to check for trade oppotunities, in seconds:

`TICK_INTERVAL_SECONDS=30`

-----------------

The URL for the Stellar Horizon API that Rockfish will query. The Stellar foundation's live Horizon server is at `"https://horizon.stellar.org/"`, though anyone (including you!) can run a Horizon instance. The below is a Horizon server for the Stellar test network. 

`HORIZON_URL="https://horizon-testnet.stellar.org"`

#### Arbitcycle

The actual settings for Rockfish:

The hold asset; this is what you need in Rockfish's trading account. For any asset that is not XLM you will also need the token issuer's account address, which you can find on any of the web front-ends for the SDEX, or on the [stellar.expert](https://stellar.expert) block explorer:

`HOLD_ASSET_CODE="XLM"`
`HOLD_ASSET_ISSUER=""`

-----------------

How much of the hold asset to pass through the trade cycles. Either have Rockfish use the account's entire hold asset balance with:

`USE_BALANCE = true`

Or set a specific amount:

`STATIC_AMOUNT = 10.0`

-----------------

What minimum amount of the hold asset Rockfish should consider sending if your specified amount is too much to make it through the cycle. This is to avoid extremely small trades that can lose money even with Stellar's tiny fees:

`MIN_AMOUNT = 1.0`

-----------------

What profit margin Rockfish should look for before it bothers sending a cycle. If you trade on centralized exchanges you may be used to fees based on the size of a trade. On the SDEX there is a flat fee of 0.00001 lumen per operation, regardless of payment or trade size. If we were doing this on a centralized exchange we'd have to account for something like a 0.1% fee per step; on the SDEX we can pursue much smaller margins.

`MIN_RATIO=1.0001`

-----------------

The assets you want to trade through, each in preceeded by `[[ASSETS]]`. You must have at least two, but the list can be as many as you like. Note the `GROUP` parameter. Rockfish will check all possible paths through the assets you list, which can add up to a *lot* of paths if you set more than a few assets. To keep this under control, set the assets that actually trade against each other to the same group number, and if there is another set of assets you want to use, set them to a different group number. Use integers here, unlike most other settings, which are decimal values.

````
[[ASSETS]]
CODE = "COUPON" 
ISSUER = "GSIGH..."  
GROUP = 1
````

### Run Rockfish

Open your command line or bash to the directory where you placed the "rockfish" binary. If you compiled from source this will be *rockfish/bin*.

Enter the start command, in this format:

`./rockfish arbitcycle --botConf /path/arbitconfig.cfg --stratConf /path/arbitcycle.cfg`

Where the `/path` strings are the full paths to your configuration files and their filenames.

You can also set the `--sim` flag, which puts Rockfish in simulation mode; it will query Horizon and output what it would have done. It is __**strongly**__ suggested that you do this before running it live to test your configuration settings.

Other optional flags are available:

`--log filename`, which sets Rockfish to output its log to a file in its directory; no need for an extension.

`--iter 1`, which tells Rockfish to run only a specific number to query cycles instead of continuing until stopped.

`--operationalBuffer 15`, which sets how much of an XLM balance to maintain to keep the account valid. This is set to 20 if the flag is not specified.

### Monitor the ratios and cycle attempts to see if you've chosen effective asset groups. 

When Rockfish attempts a cycle, Stellar will often return `[op_over_source_max]`, which means that the cost of the payment cycle is more than what Rockfish specified (i.e., you would lose money). This is a valid response and does not indicate a bug. The intended meaning is that the offers upon which the path was based have been taken or cancelled. Currently this is the most common response to Rockfish cycle attempts, and it is unclear whether the paths really are being removed that quickly or whether the network is determining path cost by a method that does not match Rockfish's price ratio methodology, see issue [2](https://github.com/Reidmcc/rockfish/issues/2). 

Note that no losing cycle will go through; either it succeeds and your balance increases or it bounces. You do pay the 0.00001 lumen network fee for a bounced transaction, but that is trivial if you're ever succeeding.

![rockfish icon long flip](https://user-images.githubusercontent.com/43561569/52517024-0c518c00-2bfa-11e9-9cd0-e2443d7868f1.png)
