<!--
Guiding Principles:

Changelogs are for humans, not machines.
There should be an entry for every single version.
The same types of changes should be grouped.
Versions and sections should be linkable.
The latest version comes first.
The release date of each version is displayed.
Mention whether you follow Semantic Versioning.

Usage:

Change log entries are to be added to the Unreleased section under the
appropriate stanza (see below). Each entry should ideally include a tag and
the Github issue reference in the following format:

* (<tag>) \#<issue-number> message

The issue numbers will later be link-ified during the release process so you do
not have to worry about including a link manually, but you can if you wish.

Types of changes (Stanzas):

"Features" for new features.
"Improvements" for changes in existing functionality.
"Deprecated" for soon-to-be removed features.
"Bug Fixes" for any bug fixes.
"Client Breaking" for breaking CLI commands and REST routes used by end-users.
"API Breaking" for breaking exported APIs used by developers building on SDK.
"State Machine Breaking" for any changes that result in a different AppState 
given same genesisState and txList.
Ref: https://keepachangelog.com/en/1.0.0/
-->

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.12.1-alpha](https://github.com/NibiruChain/nibiru/releases/tag/v0.12.1-alpha) - 2022-08-11

### CI

* [#795](https://github.com/NibiruChain/nibiru/pull/795) Integration tests run when PR is approved

### Improvements

* [#798](https://github.com/NibiruChain/nibiru/pull/798) - fix integration tests caused by PR #786
* [#801](https://github.com/NibiruChain/nibiru/pull/801) - remove unused pair constants
* [#788](https://github.com/NibiruChain/nibiru/pull/788) - add --overwrite flag to the nibid init call of localnet.sh
* [#804](https://github.com/NibiruChain/nibiru/pull/804) - bump ibc-go to v3.1.1
* [#818](https://github.com/NibiruChain/nibiru/pull/818) - fix(localnet.sh): add max leverage to vpools in genesis to fix open-position
* [#819](https://github.com/NibiruChain/nibiru/pull/819) - add golangci-linter using docker in Makefile

### Features

* [#791](https://github.com/NibiruChain/nibiru/pull/791) Add the x/oracle module
- [#813](https://github.com/NibiruChain/nibiru/pull/813) - (vpool): Expose mark price, mark TWAP, index price, and k (swap invariant) in the all-pools query 
* [#810](https://github.com/NibiruChain/nibiru/pull/810) - feat(x/perp): expose 'marginRatioIndex' and block number on QueryTraderPosition

## [v0.12.1](https://github.com/NibiruChain/nibiru/releases/tag/v0.12.1) - 2022-08-04

* [#796](https://github.com/NibiruChain/nibiru/pull/796) - fix bug that caused that epochKeeper was nil when running epoch hook from Perp module
* [#793](https://github.com/NibiruChain/nibiru/pull/793) - add a vpool parameter to limit leverage in open position

## [v0.12.0](https://github.com/NibiruChain/nibiru/releases/tag/v0.12.0) - 2022-08-03

### Improvements

* [#775](https://github.com/NibiruChain/nibiru/pull/775) - bump google.golang.org/protobuf from 1.28.0 to 1.28.1
* [#768](https://github.com/NibiruChain/nibiru/pull/768) - add simulation tests to make file
* [#767](https://github.com/NibiruChain/nibiru/pull/767) - add fluctuation limit checks on `OpenPosition`.
* [#786](https://github.com/NibiruChain/nibiru/pull/786) - add genesis params in localnet script.
* [#770](https://github.com/NibiruChain/nibiru/pull/770) - Return err in case of zero time elapsed and zero snapshots on `GetCurrentTWAP` func. If zero time has elapsed, and snapshots exists, return the instantaneous average.

### Bug Fixes

* [#766](https://github.com/NibiruChain/nibiru/pull/766) - Fixed margin ratio calculation for trader position.
* [#776](https://github.com/NibiruChain/nibiru/pull/776) - Fix a bug where the user could open infinite leverage positions
* [#779](https://github.com/NibiruChain/nibiru/pull/779) - Fix issue with released tokens being invalid in `ExitPool`

### Testing

* [#782](https://github.com/NibiruChain/nibiru/pull/782) - replace GitHub test workflows to use make commands
* [#784](https://github.com/NibiruChain/nibiru/pull/784) - fix runsim
* [#783](https://github.com/NibiruChain/nibiru/pull/783) - sanitise inputs for msg swap simulations

## [v0.11.0](https://github.com/NibiruChain/nibiru/releases/tag/v0.11.0) - 2022-07-29

### Documentation

* [#701](https://github.com/NibiruChain/nibiru/pull/701) Add release process guide

### Improvements

* [#715](https://github.com/NibiruChain/nibiru/pull/715) - remove redundant perp.Keeper.SetPosition parameters
* [#718](https://github.com/NibiruChain/nibiru/pull/718) - add guard clauses on OpenPosition (leverage and quote amount != 0)
* [#728](https://github.com/NibiruChain/nibiru/pull/728) - add dependabot file into the project.
* [#723](https://github.com/NibiruChain/nibiru/pull/723) - refactor perp keeper's `RemoveMargin` method
* [#730](https://github.com/NibiruChain/nibiru/pull/730) - update localnet script.
* [#736](https://github.com/NibiruChain/nibiru/pull/736) - Bumps [github.com/spf13/cast](https://github.com/spf13/cast) from 1.4.1 to 1.5.0
* [#735](https://github.com/NibiruChain/nibiru/pull/735) - Bump github.com/spf13/cobra from 1.4.0 to 1.5.0
* [#729](https://github.com/NibiruChain/nibiru/pull/729) - move maintenance margin to the vpool module
* [#741](https://github.com/NibiruChain/nibiru/pull/741) - remove unused code and refactored variable names.
* [#742](https://github.com/NibiruChain/nibiru/pull/742) - Vpools are not tradeable if they have invalid oracle prices.
* [#739](https://github.com/NibiruChain/nibiru/pull/739) - Bump github.com/spf13/viper from 1.11.0 to 1.12.0

### API Breaking

* [#721](https://github.com/NibiruChain/nibiru/pull/721) - Updated proto property names to adhere to standard snake_casing and added Unlock REST endpoint
* [#724](https://github.com/NibiruChain/nibiru/pull/724) - Add position fields in `ClosePositionResponse`.
* [#737](https://github.com/NibiruChain/nibiru/pull/737) - Renamed from property to avoid python name clash

### State Machine Breaking

* [#733](https://github.com/NibiruChain/nibiru/pull/733) - Bump github.com/cosmos/ibc-go/v3 from 3.0.0 to 3.1.0
* [#741](https://github.com/NibiruChain/nibiru/pull/741) - Rename `epoch_identifier` param to `funding_rate_interval`.
* [#745](https://github.com/NibiruChain/nibiru/pull/745) - Updated pricefeed twap calc to use bounded time

### Bug Fixes

* [#746](https://github.com/NibiruChain/nibiru/pull/746) - Pin cosmos-sdk version to v0.45 for proto generation.

## [v0.10.0](https://github.com/NibiruChain/nibiru/releases/tag/v0.10.0) - 2022-07-18

### Improvements

* [#705](https://github.com/NibiruChain/nibiru/pull/705) Refactor PerpKeeper's `AddMargin` method to accept individual fields instead of the entire Msg object.

### API Breaking

* [#709](https://github.com/NibiruChain/nibiru/pull/709) Add fields to `OpenPosition` response.
* [#707](https://github.com/NibiruChain/nibiru/pull/707) Add fluctuation limit checks in vpool methods.
* [#712](https://github.com/NibiruChain/nibiru/pull/712) Add funding rate calculation and `FundingRateChangedEvent`.

### Upgrades

* [#725](https://github.com/NibiruChain/nibiru/pull/725) Add governance handler for creating new virtual pools.
* [#702](https://github.com/NibiruChain/nibiru/pull/702) Add upgrade handler for v0.10.0.

## [v0.9.2](https://github.com/NibiruChain/nibiru/releases/tag/v0.9.2) - 2022-07-11

### Improvements

* [#686](https://github.com/NibiruChain/nibiru/pull/686) Add changelog enforcer to github actions.
* [#681](https://github.com/NibiruChain/nibiru/pull/681) Remove automatic release and leave integration tests when merge into master.
* [#684](https://github.com/NibiruChain/nibiru/pull/684) Reorganize PerpKeeper methods.
* [#690](https://github.com/NibiruChain/nibiru/pull/690) Call `closePositionEntirely` from `ClosePosition`.
* [#689](https://github.com/NibiruChain/nibiru/pull/689) Apply funding rate calculation 48 times per day.

### API Breaking

* [#687](https://github.com/NibiruChain/nibiru/pull/687) Emit `PositionChangedEvent` upon changing margin.
* [#685](https://github.com/NibiruChain/nibiru/pull/685) Represent `PositionChangedEvent` bad debt as Coin.
* [#697](https://github.com/NibiruChain/nibiru/pull/697) Rename pricefeed keeper methods.
* [#689](https://github.com/NibiruChain/nibiru/pull/689) Change liquidation params to 2.5% liquidation fee ratio and 25% partial liquidation ratio.

### Testing

* [#695](https://github.com/NibiruChain/nibiru/pull/695) Add `OpenPosition` integration tests.
* [#692](https://github.com/NibiruChain/nibiru/pull/692) Add test coverage for Perp MsgServer methods.
