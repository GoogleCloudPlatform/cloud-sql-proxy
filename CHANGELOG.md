# Changelog

### [1.22.1](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.22.0...v1.22.1) (2021-06-01)


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
