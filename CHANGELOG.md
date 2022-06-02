# Changelog

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
