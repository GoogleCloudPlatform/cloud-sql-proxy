# Changelog

## [2.14.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.13.0...v2.14.0) (2024-10-23)


### Features

* callback notify function when connection is refused ([#2308](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2308)) ([9309b84](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/9309b8461d73d83137943885aad164793a14a875))

## [2.13.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.12.0...v2.13.0) (2024-08-14)


### Features

* bump to Go 1.23 ([#2287](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2287)) ([fd6bd49](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/fd6bd49242c884508f641c754eb5cec5d28ac665))

## [2.12.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.11.4...v2.12.0) (2024-07-17)


### Features

* Add parameter --min-sigterm-delay allow new connections for a minimum number off seconds before shutting down the proxy. ([#2266](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2266)) ([52cd0d9](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/52cd0d95695d2b8e9456825e7c0bd452234a867b)), closes [#1640](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1640)
* add support for Debian bookworm ([#2267](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2267)) ([fbec17b](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/fbec17bd2c8c0898bdf41eb22669a871e5048ba9))


### Bug Fixes

* ignore go-fuse ctx in Lookup ([#2268](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2268)) ([ae8ec35](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/ae8ec359b43056fe815ac7c649388232bc1b4171))
* Make the process exit if there as an error accepting a fuse connection. ([#2257](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2257)) ([bb2a0ae](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/bb2a0ae76d518eaeec69fcc2ac7e930a4bd7e024))

## [2.11.4](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.11.3...v2.11.4) (2024-06-12)


### Bug Fixes

* bump dependencies to latest ([#2249](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2249)) ([6501df5](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/6501df5f34fdf82651bea163b9014ea15dc86b81))

## [2.11.3](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.11.2...v2.11.3) (2024-05-28)


### Bug Fixes

* bump dependencies to latest ([#2236](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2236)) ([14ff947](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/14ff947fa6c3b9a0023d5be7ad5b165cf6ac153b))

## [2.11.2](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.11.1...v2.11.2) (2024-05-16)


### Bug Fixes

* bump dependencies to latest ([#2218](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2218)) ([44dff63](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/44dff63c0d9ec755565ab54f1dd48e9967f6d513))

## [2.11.1](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.11.0...v2.11.1) (2024-05-14)


### Bug Fixes

* don't depend on downstream in readiness check ([#2207](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2207)) ([49fa927](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/49fa927ede69bf24f3fd0c56e60b99e4111d58f1)), closes [#2083](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2083)
* ensure proxy shutsdown cleanly on fuse error ([#2205](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2205)) ([54e65d1](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/54e65d14a5d533f44e33b52a2dc88c2a419eae2f)), closes [#2013](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2013)
* use public mirrors for base images ([#2190](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2190)) ([69b4215](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/69b42158291b0ea4f074469dabbe34949af86053))

## [2.11.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.10.1...v2.11.0) (2024-04-16)


### Features

* add support for a lazy refresh ([#2184](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2184)) ([fd7ab82](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/fd7ab82796c052ddf12f78989e5d3cab49f26c55)), closes [#2183](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2183)
* use Google managed base images ([#2159](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2159)) ([1103a95](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/1103a95adb0c0751df99704f71a4376ce38613a4))

## [2.10.1](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.10.0...v2.10.1) (2024-03-20)


### Bug Fixes

* correct CI/CD build error ([#2155](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2155)) ([3e3b8ed](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/3e3b8ede607a50ce72af8f8d1d86eb6789560aef))

## [2.10.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.9.0...v2.10.0) (2024-03-14)


### Features

* add support for config file ([#2106](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2106)) ([c936396](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/c9363966cb017cde7712426c3e9c999e3d7e0973))
* add TPC support ([#2116](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2116)) ([7d011f8](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/7d011f8f1bb87488f639a3bfde09f57ac350ab8c))


### Bug Fixes

* use kebab case for config file consistently ([#2130](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2130)) ([ee52f07](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/ee52f0759a84bad9d8cec4a3cd1f8ff536c2e982))

## [2.9.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.8.2...v2.9.0) (2024-02-20)


### Features

* add support for debug logging ([#2107](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2107)) ([c8f7a0a](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/c8f7a0abc325a9183b23710e30f5d1c9e619aef5))

## [2.8.2](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.8.1...v2.8.2) (2024-01-17)


### Bug Fixes

* update dependencies to latest versions ([#2089](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2089)) ([6d9981a](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/6d9981a757e3c1033954db7b6f834c42c5495e4f))

## [2.8.1](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.8.0...v2.8.1) (2023-12-12)


### Bug Fixes

* label container images correctly ([#2061](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2061)) ([f071d38](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/f071d38e152c70113d7102c19ed450c74e8d64f0))
* Update Go Connector to v1.5.2 to ensure connections work after waking from sleep
([#1788](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1788))

## [2.8.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.7.2...v2.8.0) (2023-12-04)


### Features

* add support for wait command ([#2041](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2041)) ([1c00ba4](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/1c00ba475374e6ae46956c4125b91a55fe953751))

## [2.7.2](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.7.1...v2.7.2) (2023-11-14)


### Bug Fixes

* use proper error instance to write error log ([#2014](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2014)) ([cc76a54](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/cc76a544a6878ad9f0ef5fb407de314b3f801cbe))

## [2.7.1](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.7.0...v2.7.1) (2023-10-17)


### Bug Fixes

* bump dependencies to latest ([#2004](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/2004)) ([4953402](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/4953402d9a63613729a2f5b8a33ac0323b7b6bb9))

## [2.7.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.6.1...v2.7.0) (2023-09-19)


### Features

* /quitquitquit api now responds to HTTP GET and POST requests. ([#1947](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1947)) ([e5ebb48](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/e5ebb485f7a7a5f9820822bf4e84467da431fc6b)), closes [#1946](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1946)
* Add support for systemd notify ([#1930](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1930)) ([cf23647](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/cf23647f72990fc3e6b4987e3040c6020929b97d))

## [2.6.1](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.6.0...v2.6.1) (2023-08-16)


### Bug Fixes

* remove the error message for zero on sigterm ([#1902](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1902)) ([55f0f60](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/55f0f60c9701a22e657ba814c9cfe6221c4840e7))

## [2.6.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.5.0...v2.6.0) (2023-07-14)


### Features

* add support for exit zero on sigterm ([#1870](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1870)) ([e0a97dd](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/e0a97ddd5bea94054b1da0c3f0844ab47ad6f126))

## [2.5.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.4.0...v2.5.0) (2023-07-11)


### Features

* add PSC support to proxy ([#1863](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1863)) ([496974a](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/496974a31555d64f144d24247507c0ec457d7edd))

## [2.4.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.3.0...v2.4.0) (2023-06-14)


### Features

* add connection test for startup ([#1832](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1832)) ([47dae85](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/47dae851a9513bb5e3d98b59a33aef909a2bf125)), closes [#348](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/348)
* allow connections during shutdown ([#1805](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1805)) ([4a456ed](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/4a456ed6a9727f672783aee021b20a208971270d))


### Bug Fixes

* log info message for quitquitquit ([#1806](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1806)) ([4d36204](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/4d36204cb6c93751e9a7d40be5e3eff94a90847f))
* Windows service stop ([#1833](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1833)) ([17e66a7](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/17e66a7a73a88d5c29c77133cdb5ad5aebd0a4c1))

## [2.3.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.2.0...v2.3.0) (2023-05-16)


### Features

* Add Windows Service support ([#1696](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1696)) ([ec56eba](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/ec56ebab683804885edd95365e099de7a0de578f)), closes [#277](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/277)


### Bug Fixes

* disallow auto-iam-authn with gcloud-auth ([#1762](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1762)) ([8200abe](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/8200abe467bdf9f5b458f108e5f086bdfbfa2dd9)), closes [#1754](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1754)

## [2.2.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.1.2...v2.2.0) (2023-04-18)


### Features

* add support for Auto IP ([#1735](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1735)) ([83c8a64](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/83c8a6444e9e1305922550bd5b6ac373babb0ffc))


### Bug Fixes

* allow `--structured-logs` and `--quiet` flags together ([#1750](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1750)) ([0aff60e](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/0aff60e5daf7995890ebc750080032bed543c9ea))
* limit calls to SQL Admin API on startup ([#1723](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1723)) ([e1a03df](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/e1a03df61120e26c7bffe86a1f971cca8bb77562))
* pass dial options to FUSE mounts ([#1737](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1737)) ([7ecf6ac](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/7ecf6ac013760a1e775db4a8da6a99a1e1817330))

## [2.1.2](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.1.1...v2.1.2) (2023-03-22)


### Bug Fixes

* update dependencies to latest versions ([#1707](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1707)) ([54ea90e](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/54ea90e140873da254a34ea8b4e612b81a46cf13))

## [2.1.1](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.1.0...v2.1.1) (2023-02-23)


### Bug Fixes

* build statically linked binaries ([#1680](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1680)) ([49308c5](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/49308c5c3c2372e4cb7e26f58c1e0dba7953f663))

## [2.1.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.0.0...v2.1.0) (2023-02-16)


### Features

* add support for Go 1.20 ([#1630](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1630)) ([72df17d](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/72df17d9a1992d51faf3a9f4ecd3960f680b7ef3))
* add support for quitquitquit endpoint ([#1624](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1624)) ([43f9857](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/43f98574de06211581779a67806b01d5518cdd62))
* Add unix-socket-path to instance command arguments. ([#1623](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1623)) ([f42f3d1](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/f42f3d1ce9fc81b78d9e8bd68b147cae20516fae)), closes [#1573](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1573)


### Bug Fixes

* ensure separate token source with auto-iam-authn ([#1637](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1637)) ([325a487](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/325a487187c9a9cb1864b8f387b1b06369e1ca25))
* honor request context in readiness check ([#1657](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1657)) ([0934739](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/09347395eddd8b4942a1cfb77344b014c8bdc90b))

## [2.0.0](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.0.0-preview.4...v2.0.0) (2023-01-17)


### Bug Fixes

* correctly apply metadata to user agent ([#1606](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1606)) ([1ca9902](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/1ca9902fb949ea2a75fcdc5ed9930877db6ff600))


### Miscellaneous Chores

* release 2.0.0 ([#1615](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1615)) ([4a6283b](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/4a6283b70b49f97a5b60ebb68c9e01d6add2dff0))

## [2.0.0-preview.4](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.0.0-preview.3...v2.0.0-preview.4) (2022-12-12)


### Features

* add admin server with pprof ([#1564](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1564)) ([d022c56](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/d022c5683a301722e55692ae3ca1d62cf0e6d017))


### Bug Fixes

* add runtime version to user agent if present ([#1542](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1542)) ([a6b689b](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/a6b689b05aa6f1e11ede8a1dd6fdec3cfc3c8c8e))
* use user-agent as flag name ([#1561](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1561)) ([e1b2f7e](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/e1b2f7eb4e9552a1124b7aab3a1bca4366797b53))


### Miscellaneous Chores

* release 2.0.0-preview.4 ([#1576](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1576)) ([04fcf88](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/04fcf88d35a1da7e623b763829a02e79431fb74e))

## [2.0.0-preview.3](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.0.0-preview.2...v2.0.0-preview.3) (2022-11-15)


### Features

* add quiet flag ([#1515](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1515)) ([93d9a40](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/93d9a40cf8736bfce2d3cc6bc20b2defafa0413f)), closes [#1452](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1452)
* add support for min ready instances ([#1496](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1496)) ([73e2999](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/73e2999f3f3da32a63149a2d0cc08750038f721f))
* configure the proxy with environment variables ([#1514](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1514)) ([2a9d9a2](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/2a9d9a2cd93804818a5659ee554c56336969d861))


### Bug Fixes

* correct bullseye Dockerfile ([#1504](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1504)) ([15a97e7](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/15a97e7f8287098232719cfae2a2ad6242a7a92a))
* correct error check in check connections ([#1505](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1505)) ([776a86b](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/776a86b15f14d1c0846f9813845829ffb9642bb8))
* impersonated user uses downscoped token ([#1520](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1520)) ([b08c71d](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/b08c71d02ea7c33c587a0a75c30e06242029028b))
* return correct exit code on SIGTERM ([#1530](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1530)) ([7bb15aa](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/7bb15aa482c2278f6e49a3a0f4a7baf4a2f4b511))


### Miscellaneous Chores

* release 2.0.0-preview.3 ([#1548](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1548)) ([024963b](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/024963b9b8b38fbb8b2d7043be5a5658a77689c0))

## [2.0.0-preview.2](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.0.0-preview.1...v2.0.0-preview.2) (2022-10-25)


### Features

* add bullseye container image ([#1468](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1468)) ([36a0172](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/36a01725b6d1ef30450570d3780871521a3ed6f3))
* add support for impersonation ([#1460](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1460)) ([d0f8e55](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/d0f8e55ccb390b9c1f803b3a6c4f2e7874f40337)), closes [#417](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/417)
* add support for JSON credentials flag ([#1433](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1433)) ([2a9c8d8](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/2a9c8d8cb24e2c84e43620dab333677191d1dbd7))
* bump to Go 1.19 ([#1411](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1411)) ([02e008a](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/02e008a6d886a461a76ddc899c8891621ca2f58c))
connector/commit/bd20b6bfe746cfea778b9e1a9702de28047e5950))
* cloud.google.com/go/cloudsqlconn: Downscope OAuth2 token included in ephemeral certificate ([#​332](https://togithub.com/GoogleCloudPlatform/cloud-sql-go-connector/issues/332)) ([d13dd6f](https://togithub.com/GoogleCloudPlatform/cloud-sql-go-connector/commit/d13dd6f3e7db0179511539315dec1c2dc96f0e3e))


### Bug Fixes

* don't build FUSE paths for Windows ([#1400](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1400)) ([be2d14f](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/be2d14f39fa88f17cf69cf338719f08d2f81143b))
* restore openbsd and freebsd support ([#1442](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1442)) ([05dcdd4](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/05dcdd4b48bc6fabba14ae41dcc5de7c1d0c3f2f))
* set write permissions for group and other ([#1405](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1405)) ([f6b77d7](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/f6b77d7b42633f689be877e469173fa42a6877a8))
* cloud.google.com/go/cloudsqlconn: throw error when Auto IAM AuthN is unsupported ([#​310](https://togithub.com/GoogleCloudPlatform/cloud-sql-go-connector/issues/310)) ([652e196](https://togithub.com/GoogleCloudPlatform/cloud-sql-go-connector/commit/652e196b427ce9673676e214c6ad3905b21a68b0))


### Miscellaneous Chores

* release 2.0.0-preview.2 ([#1503](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1503)) ([67345e9](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/67345e9b7986d0951f73ecdfefb5f0d7ef2eef18))

## [2.0.0-preview.1](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/compare/v2.0.0-preview.0...v2.0.0-preview.1) (2022-09-07)


### Features

* add support for FUSE ([#1381](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1381)) ([6cf4d25](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/6cf4d255fe7d640db4e7e651aed9c377ecf4e735))


### Bug Fixes

* pass dial options when checking connections ([#1366](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1366)) ([0033c36](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/0033c36200e7b5ba77b8d1157b1168af5fba73fc))
* support configuration of HTTP server address ([#1365](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/issues/1365)) ([b53d77f](https://github.com/GoogleCloudPlatform/cloud-sql-proxy/commit/b53d77fce751af9316a4bc1349cd3bbcaaf151b0))

## [1.31.2](https://github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.31.1...v1.31.2) (2022-08-02)


### Bug Fixes

* update dependencies to latest versions ([#1286](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1286)) ([d3f9dcb](https://github.com/GoogleCloudPlatform/cloudsql-proxy/commit/d3f9dcbe81bb43a0602e35359a262b2920f1915e))

## [1.31.1](https://github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.31.0...v1.31.1) (2022-07-12)


### Bug Fixes

* strip monotonic clock reading during refresh ([#1223](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1223)) ([957d160](https://github.com/GoogleCloudPlatform/cloudsql-proxy/commit/957d1609ad96bfed77b3744f1c11a762010bc06e))

## [1.31.0](https://github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.30.1...v1.31.0) (2022-06-02)


### Features

* make Docker images ARM-friendly ([#1193](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1193)) ([6a98a04](https://github.com/GoogleCloudPlatform/cloudsql-proxy/commit/6a98a0407785db7085532ea242b7079ceba756e3))

### [1.30.1](https://github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.30.0...v1.30.1) (2022-05-03)


### Bug Fixes

* update dependencies to latest versions ([#1187](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1187)) ([f915180](https://github.com/GoogleCloudPlatform/cloudsql-proxy/commit/f9151809664e1847db94b0e4da905aece000d8fa))

## [1.30.0](https://github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.29.0...v1.30.0) (2022-04-04)


### Features

* drop support and testing for Go 1.13, 1.14, 1.15 ([#1148](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1148)) ([158b0d5](https://github.com/GoogleCloudPlatform/cloudsql-proxy/commit/158b0d57d46054be6a0d1600d5030b23be69dc9b))

## [1.29.0](https://github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.28.1...v1.29.0) (2022-03-01)


### Features

* add Go version support policy ([#1109](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1109)) ([ae6f4a1](https://github.com/GoogleCloudPlatform/cloudsql-proxy/commit/ae6f4a1a534df8a273c0ea96880154b90bc65e77))

### [1.28.1](https://github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.28.0...v1.28.1) (2022-01-31)


### Bug Fixes

* invalidated config should retain error ([#1068](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1068)) ([49d3003](https://github.com/GoogleCloudPlatform/cloudsql-proxy/commit/49d3003c018afdc0cde54340d5be808f9dcd5c84))
* remove unnecessary token parsing ([#1074](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1074)) ([e138611](https://github.com/GoogleCloudPlatform/cloudsql-proxy/commit/e1386118ad239e6c1ff16df6f2be1351a6432bb3))
* return error from instance version ([#1069](https://github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1069)) ([d9fc819](https://github.com/GoogleCloudPlatform/cloudsql-proxy/commit/d9fc819a197bd75d0060bd46b8e06da6bdd6630c))

## [1.28.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.27.1...v1.28.0) (2022-01-04)


### Features

* add support for ReadTime in Admin API requests ([#1040](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1040)) ([a7c8b5c](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/a7c8b5cf4d10c17bea405ce67ee642232b43fdec))
* add support for specifying a quota project ([#1044](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1044)) ([dc66aca](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/dc66aca88190ae3f6d39f191489fdfb280146ed9))
* allow multiple -instances flags ([#1046](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1046)) ([1972693](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/1972693b8ac65c912bb719dc23d4f578cb6ff9e2)), closes [#1030](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1030)


### Bug Fixes

* increase rateLimit burst size to 2 ([#1048](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1048)) ([df6b6f9](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/df6b6f9ed8860d28f5e934db495257d288c42f2b))

### [1.27.1](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.27.0...v1.27.1) (2021-12-07)


### Bug Fixes

* update dependencies to latest versions ([#1034](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/1034)) ([8954d24](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/8954d241a71b59d9bf82cb47469e6652d3f379e7))

## [1.27.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.26.0...v1.27.0) (2021-11-02)


### Features

* switch to supported FUSE library ([#953](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/953)) ([10f2133](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/10f2133010f3bf7ef8a13b43e0bfa16bdca8cedb))
* verify FUSE is installed on macOS / linux ([#959](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/959)) ([9ab868e](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/9ab868ef344b9a82c06f97928420f98a4d37c5ce))


### Bug Fixes

* fail fast on invalid config ([#999](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/999)) ([18a0960](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/18a096037d9ceb2ca71218984b65fe342fc2a778))
* respect context deadline for TLS handshakes ([#987](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/987)) ([12ff12c](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/12ff12c9f87459dc40e2e6e4a2d08bebb0786ee7)), closes [#986](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/986)
* validate instance connections in liveness probe ([#995](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/995)) ([e5cc8d4](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/e5cc8d4f8676fed2013cc491578a1aaf7416ec3e))

## [1.26.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.25.0...v1.26.0) (2021-10-05)


### Features

* improve reliability of refresh operations ([#883](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/883)) ([480992a](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/480992a7671abe9b76f940175f4ed17f5271d3f8))

## [1.25.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.24.0...v1.25.0) (2021-09-07)


### Features

* add health checks to proxy ([#859](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/859)) ([ea62bdd](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/ea62bddaaf3aa7df79250d045ba2f5f3fe7edaea))
* add instance dialing to health check ([#871](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/871)) ([eca3793](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/eca37935e7cd54efcd612c170e46f45c1d8e3556))
* require TLS v1.3 at minimum ([#906](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/906)) ([cafa966](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/cafa966e50170ad94f12f067549ba3aedf8ecdca))


### Bug Fixes

* ensure proxy shuts down gracefully on SIGTERM ([#877](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/877)) ([9793555](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/97935551ac44cb7a92e2901def1938d604dfeecb))
* validate instances in fuse mode ([#875](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/875)) ([96f8b65](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/96f8b655b09b711fd9adfcb486626b64d3b917f3))

## [1.24.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.23.1...v1.24.0) (2021-08-02)


### Features

* Add option to delay key generation until first connect ([#841](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/841)) ([4999ffd](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/4999ffd0c3406e91874648630f9805b2d5f0ac50))
* stop building darwin 386 binaries ([#846](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/846)) ([77d7c40](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/77d7c40ff79cf99a10d2dbae39b737625a08582f)), closes [#780](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/780)


### Bug Fixes

* invalidate cached config on handshake error ([#817](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/817)) ([5d98f5c](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/5d98f5c40e0b58da479bf6897712d53e6846f613))
* strip padding from access tokens if present ([#851](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/851)) ([1f195e5](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/1f195e500c1a8989dcf4d73c429620ddd5b20891))
* structured_logs compatibility with Google Cloud Logging ([#861](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/861)) ([74a6ec7](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/74a6ec70b63f4f0488470164fa4da68a26779fb2))

### [1.23.1](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.23.0...v1.23.1) (2021-07-12)


### Bug Fixes

* improve log message when refresh is throttled ([#830](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/830)) ([4ffee2a](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/4ffee2a1950fd6fb6703647d178a436b566b8a80))

## [1.23.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.22.0...v1.23.0) (2021-06-01)


### Features

* add deprecation warning for Darwin 386 ([#781](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/781)) ([cdc552b](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/cdc552b8da7abb3378d43c060acb019de7e12fcc))


### Bug Fixes

* change to static base container ([#791](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/791)) ([d66233e](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/d66233e2a0aecb6e80a4f802b0dc6a5cd2fa9041))

## [1.22.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.21.0...v1.22.0) (2021-04-21)


### Features

* Add support for systemd notify ([#719](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/719)) ([4305eff](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/4305eff05f1d33da4251a7b512b723cb086e4ce5))


### Bug Fixes

* Allow combined use of structured logs and -log_debug_stdout ([#726](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/726)) ([45bda77](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/45bda776fc964a3464a1703035b4f2a719779bc6))
* return early when cert refresh fails ([#748](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/748)) ([fd21f66](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/fd21f66f2d8dc3b8e787ab0b467db4d4b85921cb))
* structured logging respects the -verbose flag ([#737](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/737)) ([f35422f](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/f35422f449a0c79f6b2225de21c26c2da04d3528))

## [1.21.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.20.2...v1.21.0) (2021-04-05)


### Features

* add support for structured logs ([#650](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/650)) ([ca8993a](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/ca8993a2110affa0b0cbbfdebf6f6bdd86004e9f))


### Bug Fixes

* improve cache to prevent multiple concurrent refreshes ([#674](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/674)) ([c5ffa69](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/c5ffa69952eba713e7acc688841f9b448a180625))
* lower refresh buffer and config throttle when IAM authn is enabled ([#680](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/680)) ([58acab3](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/58acab3b03375032501f17c85949db493af7a292))
* prevent refreshCfg from scheduling multiple refreshes ([#666](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/666)) ([52db349](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/52db3492ac78a9a68218c2a12840c4016b1d0b99))

### [1.20.2](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.20.1...v1.20.2) (2021-03-05)


### Bug Fixes

* ensure certificate expiration is correct ([#659](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/659)) ([2fd2504](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/2fd2504381405b0d5fe7cc81d3c55a15f949df99))
* perform initial gcloud check and reuse token ([#657](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/657)) ([f3bf3f9](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/f3bf3f931621285875363fab5fe3563bc82a3d94))

### [1.20.1](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.20.0...v1.20.1) (2021-03-04)


### Bug Fixes

* prevent untrusted gcloud exe's from running ([#649](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/649)) ([0f0ff49](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/0f0ff49a0fac990ba1ec05a6cbd4e666e3141c08))
* use new oauth2 token with cert refresh ([#648](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/648)) ([6d5e455](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/6d5e4558a63957714f6347c9768e671586c0a605))
* verify TokenSource exists in TokenExpiration() ([#642](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/642)) ([d01d7eb](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/d01d7eb78652cf83f713b5d47bb696378929e8a6))

## [1.20.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.19.2...v1.20.0) (2021-02-24)


### Features

* add ARM releases ([#631](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/631)) ([d3fb7f6](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/d3fb7f6394f2c641f0ba7339ab29a1c02d82e396))
* Added '-enable_iam_login' flag for IAM db authentication ([#583](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/583)) ([470f92d](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/470f92d29d7a32f7903a3cb6d49fb09363185866))


### [1.19.2](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.19.1...v1.19.2) (2021-02-16)


### Bug Fixes

* improve logging for file descriptor limits ([#609](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/609)) ([b42b681](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/b42b68134543fbee7da4fbb9a8d667fd9153bec2)), closes [#413](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/413)

### [1.19.1](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.19.0...v1.19.1) (2020-12-02)


### Bug Fixes

* Ensure necessary fields are 64-bit aligned ([#550](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/550)) ([4575c8f](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/4575c8f8cb496ac3069208e446c47fb6c6acb868))

## [1.19.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.18.0...v1.19.0) (2020-11-18)


### Features

* Added DialContext to Client and proxy package ([#483](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/483)) ([c84aa50](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/c84aa5079668e07e3d2dc8f254d30e1103a6ead3))
* use regionalized instance ids to prevent global conflicts with sqladmin v1 ([#504](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/504)) ([6c45513](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/6c455136a24b841dbfc015a1f8ed7505f9e77dec))


### Bug Fixes

* **containers:** Allow non-root users to mount fuse filesystems for alpine and buster images ([#540](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/540)) ([5b653f5](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/5b653f5df6d9c4c226e3c4f6036d5e7d4c43c699))
* only allow fuse mode to unmount if an error occurs first ([#537](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/537)) ([6caef36](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/6caef36968d23b931c824450e418e29ac6277191))
* refreshCfg no longer caches error over valid cert ([#521](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/521)) ([4a6b3d8](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/4a6b3d8c895e2634afd8cee2341db668f20b9a33))

## [1.18.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.17.0...v1.18.0) (2020-09-08)


### Features

* **containers:** Add "-alpine" and "-buster" based images.  ([#415](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/415)) ([ebcf294](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/ebcf294b9ee028340695868fb6f4cc4bbe09d849))
* **containers:** Add fuse to alpine and buster images ([#459](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/459)) ([0f28fcd](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/0f28fcd008a5bb863ec2ca1402c31ae81d7dae5d))


### Bug Fixes
* Print out any errors during SIGTERM-caused shutdown ([#389](https://github.com/GoogleCloudPlatform/cloudsql-proxy/pull/389))
* Optimize `-term-timeout` wait ([#391](https://github.com/GoogleCloudPlatform/cloudsql-proxy/pull/391))
* Add socket suffix for Postgres instances when running in `-fuse` mode ([#426](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/426)) ([20ffaec](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/20ffaec2f0f00a2516206a0453bd0d1c6e62770c))
* **containers:** Specify nonroot user by uid to work with runAsNonRoot ([#402](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/402)) ([c5c0be1](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/c5c0be1b60bfc1c3fa862039619908a328066e5e))
* Releases are now tagged using `vMAJOR.MINOR.PATCH` for correct compatibility with go-modules. Please note that this will effect container image tags (which were previously only `vMAJOR.MINOR`), since these tags correspond directly to the release on GitHub.
