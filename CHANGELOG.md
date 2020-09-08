# Changelog

## [1.18.0](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/compare/v1.17.0...v1.18.0) (2020-09-08)


### Features

* **containers:** Add "-alpine" and "-buster" based images.  ([#415](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/415)) ([ebcf294](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/ebcf294b9ee028340695868fb6f4cc4bbe09d849))
* **containers:** Add fuse to alpine and buster images ([#459](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/459)) ([0f28fcd](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/0f28fcd008a5bb863ec2ca1402c31ae81d7dae5d))


### Bug Fixes

* Add socket suffix for Postgres instances when running in `-fuse` mode ([#426](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/426)) ([20ffaec](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/20ffaec2f0f00a2516206a0453bd0d1c6e62770c))
* **container:** Specify nonroot user by uid to work with runAsNonRoot ([#402](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/402)) ([c5c0be1](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/c5c0be1b60bfc1c3fa862039619908a328066e5e))
* **docs:** Update helm chart URL ([#411](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/411)) ([60bab64](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/60bab6481d784761d0b8c36a0ee8b6d53db250f9))
* **examples:** Remove sidecar from no-proxy example. ([#410](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/410)) ([5f761d7](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/5f761d7ef539bfe4fb65c6856d439496cddbfcc7))
* **examples/k8s:** Make secret keys consistent ([#405](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/issues/405)) ([54573d5](https://www.github.com/GoogleCloudPlatform/cloudsql-proxy/commit/54573d521428a322f8049b117854987830fa082a))
