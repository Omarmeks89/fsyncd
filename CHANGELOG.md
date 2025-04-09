<!-- insertion marker -->
## Unreleased

<small>[Compare with latest](https://github.com/Omarmeks89/fsyncd/compare/5d8003c1e1ee6840838a70861f9c5d43f3986b87...HEAD)</small>

### Build

- create Makefile ([8693bc7](https://github.com/Omarmeks89/fsyncd/commit/8693bc7ba77138ce8a5c3dd326ddc9f64d90682e) by Егор Марков).

### Bug Fixes

- fix bug with copy-paste, the reason of files not created ([36703ae](https://github.com/Omarmeks89/fsyncd/commit/36703ae18ecc26c5c5cbeb05d81197b01fa58f4e) by Егор Марков).
- fix bugs with goro pool, directory creation ([2a1f00e](https://github.com/Omarmeks89/fsyncd/commit/2a1f00e494d00f38f7db9544cfaaad8eb668bc67) by Егор Марков). test: fix tests, add new tests
- fix bug with dst path shadowing (with src path) ([984d47b](https://github.com/Omarmeks89/fsyncd/commit/984d47b375712804105de44b4692542fad65636e) by Егор Марков).
- fix Compare implementation ([b80cc6d](https://github.com/Omarmeks89/fsyncd/commit/b80cc6d96febe4253f9bc8687f9d1a29d837d4c1) by Егор Марков). test: fix broken tests

### Features

- create config, create server, update logger setup, update main func, create configuration file ([386ff90](https://github.com/Omarmeks89/fsyncd/commit/386ff901d367d7230e1329bde8d665cfc4d1797e) by Егор Марков).
- create Sync function as a command entry point ([34bd980](https://github.com/Omarmeks89/fsyncd/commit/34bd980826fcb784bffce54f4782b9063fdf357c) by Егор Марков).
- create BFS creation alg to create missed directories in dst ([d6c276f](https://github.com/Omarmeks89/fsyncd/commit/d6c276f71ec29d5e0d8425274834a3c685b705fb) by Егор Марков). test: fix previous tests, add new tests
- update PrepareRootPath - add re suffix and prefix detection ([b29de56](https://github.com/Omarmeks89/fsyncd/commit/b29de569c67f26574b5be54d9039637928c6a366) by Егор Марков). test: add test for updated PrepareRootPath method
- update SyncCommand, add path creation handling ([77ff839](https://github.com/Omarmeks89/fsyncd/commit/77ff8399370500edcf6dd2871a60063a64f333ff) by Егор Марков). test: add new tests for path creation, fix prev tests
- add log.go for application logger ([ca6edba](https://github.com/Omarmeks89/fsyncd/commit/ca6edba520b838c20cd8148f93d86edc5fb79ded) by Егор Марков). test: fix tests
- add Sync() implementation ([6a37634](https://github.com/Omarmeks89/fsyncd/commit/6a37634d0a78426d5c8a480eab85441c63a76d74) by Егор Марков). test: create tests for Prepare() method, fix previous tests
- init project ([32d0157](https://github.com/Omarmeks89/fsyncd/commit/32d0157e23ffd6a5c09b0e74b3b8c844d56561d4) by Егор Марков).

### Code Refactoring

- cleanup code ([9b5cdd6](https://github.com/Omarmeks89/fsyncd/commit/9b5cdd620a66684fc64c05682e9f70791423c9e9) by Егор Марков).
- update SyncCommand implementation, delete unuseful code, delete unuseful tests ([4b5e627](https://github.com/Omarmeks89/fsyncd/commit/4b5e627c1f21e928c0f293986a1a0c84e7a77df2) by Егор Марков).

### Tests

- add tests for SyncTimeParser, fix previous tests ([a99987c](https://github.com/Omarmeks89/fsyncd/commit/a99987c9f8991a8ab0823135f36a757c95e25761) by Егор Марков).
- add test for signal error check, fix previous tests ([88f5b7f](https://github.com/Omarmeks89/fsyncd/commit/88f5b7ffc85eb9bc018d735e07cada8b7837268c) by Егор Марков).

<!-- insertion marker -->
