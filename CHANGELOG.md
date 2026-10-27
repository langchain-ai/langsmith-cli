# Changelog

## [0.3.8](https://github.com/langchain-ai/langsmith-cli/compare/v0.3.7...v0.3.8) (2026-10-27)


### Performance Improvements

* **cli:** match the API's max page size for trace messages ([b127e53](https://github.com/langchain-ai/langsmith-cli/commit/b127e53cae967bcced3292b1b3375838b755e2cd))

## [0.3.7](https://github.com/langchain-ai/langsmith-cli/compare/v0.3.6...v0.3.7) (2026-10-27)


### Features

* **cli:** expose online-evaluators bulk-delete to the CLI ([cc038c4](https://github.com/langchain-ai/langsmith-cli/commit/cc038c4d25c2332369004ed75b3ef8501d57239a))
* **cli:** expose workspaces retrieve, update and delete to the CLI ([dfbc670](https://github.com/langchain-ai/langsmith-cli/commit/dfbc670bcacefc6b2b600a93bd48d7bb2e30e1a6))
* **cli:** mount generated operations under readable command names ([6b2ff85](https://github.com/langchain-ai/langsmith-cli/commit/6b2ff851d245676c9378b1781194b62aa10d1309))
* **sdks:** stream v2 run, trace and thread queries ([90d3920](https://github.com/langchain-ai/langsmith-cli/commit/90d3920cee0d10ef0216f35a8c7292a6776fa5e3))


### Bug Fixes

* **cli:** confirm every generated command that deletes ([dab7065](https://github.com/langchain-ai/langsmith-cli/commit/dab7065cf1334cca57275f2074c883bff7af1090))

## [0.3.6](https://github.com/langchain-ai/langsmith-cli/compare/v0.3.5...v0.3.6) (2026-10-26)


### Features

* **cli:** run the connector image from the langsmith CLI ([b9a474c](https://github.com/langchain-ai/langsmith-cli/commit/b9a474c78089e88e865c2cdfe272dd0508cb809f))
* **sandboxes:** accept proxy_config when creating a browser ([2760b75](https://github.com/langchain-ai/langsmith-cli/commit/2760b75a74cdde890444cc3d6daea6efdb4bdc8e))
* **sandboxes:** embed token-mode service URLs and browser windows with the token kept out of server-visible URLs ([5e69fde](https://github.com/langchain-ai/langsmith-cli/commit/5e69fde0f27343a524aed291c217c2bc234dc891))


### Bug Fixes

* **cli:** pick up langsmith-go v0.29.1's Windows OAuth refresh fixes ([22430ce](https://github.com/langchain-ai/langsmith-cli/commit/22430ce0258a8c69ec69bba23784b2daefcaab1e))


### Documentation

* remove the beta label from agent addressing ([0cd9984](https://github.com/langchain-ai/langsmith-cli/commit/0cd9984ba4f451cab5d2ee1a89b3b42f0e4a3295))
* **threads:** bring POST /api/v2/threads/query into API standards compliance ([b08f274](https://github.com/langchain-ai/langsmith-cli/commit/b08f274486983354cf953354c403db21e2645612))

## [0.3.5](https://github.com/langchain-ai/langsmith-cli/compare/v0.3.4...v0.3.5) (2026-10-24)


### Features

* **cli:** add tunnel connect and relay connect commands ([14f997c](https://github.com/langchain-ai/langsmith-cli/commit/14f997cf946ee9789a0b123888dbb5ed205989d3))
* **cli:** expose annotation queue operations to the CLI ([ae35f27](https://github.com/langchain-ai/langsmith-cli/commit/ae35f27afd88d76393764f6d3cedec2ba3060d96))
* **cli:** expose datasets clone to the CLI ([52517cb](https://github.com/langchain-ai/langsmith-cli/commit/52517cb347e79d76c5ae84832a591751c51823e4))
* **cli:** expose datasets share to the CLI ([e7d3416](https://github.com/langchain-ai/langsmith-cli/commit/e7d341654544693263116e5198e47dc792fd7f57))
* **cli:** expose datasets splits to the CLI ([8dcc43c](https://github.com/langchain-ai/langsmith-cli/commit/8dcc43c9105c8be39d451f642f23a3135957a13a))
* **cli:** expose datasets update to the CLI ([3657496](https://github.com/langchain-ai/langsmith-cli/commit/36574969a2c8daa8ecff136a51a66246afc755c7))
* **cli:** expose datasets update-tags to the CLI ([bedffec](https://github.com/langchain-ai/langsmith-cli/commit/bedffec0ad19c7bfa9e9a5a6a9c5bd58b4693e93))
* **cli:** expose examples operations to the CLI ([9970547](https://github.com/langchain-ai/langsmith-cli/commit/997054738185b941aee1b9ac16318e9069936a7e))
* **cli:** expose feedback operations to the CLI ([e85dcb3](https://github.com/langchain-ai/langsmith-cli/commit/e85dcb3f7162c0ec8323c1b6ff74cbb70e0bb7a4))
* **cli:** expose online evaluator operations to the CLI ([68f4eaa](https://github.com/langchain-ai/langsmith-cli/commit/68f4eaa46247da16e70ce5d6756db6fd12b41a74))
* **cli:** expose product feedback retrieval to the CLI ([363dfe5](https://github.com/langchain-ai/langsmith-cli/commit/363dfe5e2c1763f73fcb3eb025fe820dc8cdc50c))
* **cli:** expose public shared-run reads to the CLI ([2b288c6](https://github.com/langchain-ai/langsmith-cli/commit/2b288c67d61500453a850414be4bcaa738b63649))
* **cli:** expose remaining sessions operations to the CLI ([66bdefb](https://github.com/langchain-ai/langsmith-cli/commit/66bdefb78c746de094ade657da2094de88de000f))
* **cli:** expose sandbox operations to the CLI ([545270e](https://github.com/langchain-ai/langsmith-cli/commit/545270ebb7ebb30bbb88830f3a234c9a0a42defc))
* **cli:** expose sandboxes registries update and browser attachments create ([a5bdce8](https://github.com/langchain-ai/langsmith-cli/commit/a5bdce878519ecfa474d42869e3ef566b7288ccb))
* **cli:** expose server info to the CLI ([e5aac90](https://github.com/langchain-ai/langsmith-cli/commit/e5aac901eb3e37dde288baf43762efd2a12f0aa6))
* **cli:** expose workspace admin operations to the CLI ([315175a](https://github.com/langchain-ai/langsmith-cli/commit/315175aa0d9ec6dce32010b423553179ecd3f5b0))
* **cli:** expose workspace settings to the CLI ([61725e4](https://github.com/langchain-ai/langsmith-cli/commit/61725e42b71c818b2cae21dacd07a2bcc85adb92))
* **cli:** let new users sign up from the CLI ([1dc89dd](https://github.com/langchain-ai/langsmith-cli/commit/1dc89dde8d624a7aea42a4dce5d94eb42b351ddc))
* **cli:** make --project optional for traces get on v1 deployments ([83ebfd9](https://github.com/langchain-ai/langsmith-cli/commit/83ebfd90cf3c2b610d8a3053f7445686549e605b))
* identify CLI requests with version, command and agent headers ([a1710d4](https://github.com/langchain-ai/langsmith-cli/commit/a1710d4e3702c18a5261eb3364fbc8c0bb89d6da))
* identify CLI requests with version, command and agent headers ([913b8c6](https://github.com/langchain-ai/langsmith-cli/commit/913b8c6813d7f18f51c35ef10d899a27c0ef9590))
* let Engine boards verify without requesting a preview ([adb8711](https://github.com/langchain-ai/langsmith-cli/commit/adb87116839b40aaeb55546b4e39748fb87aea8f))
* merge generated commands into hand-written ones and nest subresources (CLI-51) ([c5c083a](https://github.com/langchain-ai/langsmith-cli/commit/c5c083a4a83b82ee2f6d665bc52f2b7280e2acd5))
* rename CLI feedback to langsmith report (CLI-52) ([f7fa10c](https://github.com/langchain-ai/langsmith-cli/commit/f7fa10c0003f19baf948a382b109498130e94e90))
* rename CLI feedback to langsmith report (CLI-52) ([3c0cd2b](https://github.com/langchain-ai/langsmith-cli/commit/3c0cd2b53573181f11c3dd8240b364725b1161c9))


### Bug Fixes

* **examples:** keep sending outputs until servers accept reference_outputs ([26d9989](https://github.com/langchain-ai/langsmith-cli/commit/26d9989c9c7d84aa3a1d41cc6ab330a360f25329))
* make generated mounting order-independent and stricter (CLI-51) ([a030469](https://github.com/langchain-ai/langsmith-cli/commit/a0304694badb901a3329ff14897e03e860d78282))
* panic on nesting under a generated command, check op aliases (CLI-51) ([2f564d1](https://github.com/langchain-ai/langsmith-cli/commit/2f564d1b68193fce2edfeb31aefb4c19aa9370f0))
* **sandbox:** use ProxyConfigParam for --proxy-config ([157bd91](https://github.com/langchain-ai/langsmith-cli/commit/157bd911b24d63c7856c7e13bfc7e9708266eebf))


### Chores

* **deps:** bump actions/create-github-app-token from 3.2.0 to 3.3.0 ([22845fd](https://github.com/langchain-ai/langsmith-cli/commit/22845fda9e07feb32af06da36d2f27771f9b7012))
* **deps:** bump actions/create-github-app-token from 3.2.0 to 3.3.0 ([62d3df5](https://github.com/langchain-ai/langsmith-cli/commit/62d3df56241ce2b679a4bd7981ca751085f41975))
* **deps:** bump github.com/tidwall/gjson from 1.19.0 to 1.20.0 ([6103695](https://github.com/langchain-ai/langsmith-cli/commit/6103695b3abb6ce7dd5b77e71569a35728165217))
* **deps:** bump github.com/tidwall/gjson from 1.19.0 to 1.20.0 ([7d690f3](https://github.com/langchain-ai/langsmith-cli/commit/7d690f39f62073f9e83adf9bc7f83dcc7388bdf7))
* **deps:** bump github.com/tidwall/pretty from 1.2.1 to 1.2.2 ([ef5949a](https://github.com/langchain-ai/langsmith-cli/commit/ef5949a0876ca7abec69aef9932622baee60468b))
* **deps:** bump github.com/tidwall/pretty from 1.2.1 to 1.2.2 ([9a5a94b](https://github.com/langchain-ai/langsmith-cli/commit/9a5a94b1c1556a16a0ff03868628935ca03dbc6e))
* **deps:** bump goreleaser/goreleaser-action from 7.2.3 to 7.3.0 ([63fe63d](https://github.com/langchain-ai/langsmith-cli/commit/63fe63d11ccdd1c796b4e8cd81a01cd6486e0734))
* **deps:** bump goreleaser/goreleaser-action from 7.2.3 to 7.3.0 ([c74d599](https://github.com/langchain-ai/langsmith-cli/commit/c74d59974128fc7ac55e119845a800e638a89b73))
* sync generated commands ([a065b3b](https://github.com/langchain-ai/langsmith-cli/commit/a065b3b06200720a6a076e0f45696bc51417aec7))
* sync generated commands ([2ed5174](https://github.com/langchain-ai/langsmith-cli/commit/2ed51744716244b5323f1db2c61a7f1cc736023e))
* sync generated commands ([6829c3b](https://github.com/langchain-ai/langsmith-cli/commit/6829c3b024aaebb29b0ae52b17dea57245df5468))
* sync generated commands ([bc04a47](https://github.com/langchain-ai/langsmith-cli/commit/bc04a478381dcf9fd604eaa4acfcd60245925547))
* sync generated commands ([8b422fc](https://github.com/langchain-ai/langsmith-cli/commit/8b422fc9e4e007707111c256a44efa98b598e05f))
* sync generated commands ([a8e99bb](https://github.com/langchain-ai/langsmith-cli/commit/a8e99bb3d9636c4255d569250bd88c49ee63a026))
* sync generated commands ([ab9e526](https://github.com/langchain-ai/langsmith-cli/commit/ab9e52635ff8323cd29e74aec816d6fa8992152f))
* sync generated commands ([7553c21](https://github.com/langchain-ai/langsmith-cli/commit/7553c21e021e0174b65e09f771daa5e9539e985e))
* sync generated commands ([b53cea9](https://github.com/langchain-ai/langsmith-cli/commit/b53cea9ab933c5461354007f0e044a58ec3e0243))
* sync generated commands ([a154150](https://github.com/langchain-ai/langsmith-cli/commit/a15415009ad0c2447672a0feaf89c91fb74ea71e))
* sync generated commands ([8095965](https://github.com/langchain-ai/langsmith-cli/commit/80959653985ddcbcf3a6a812a4c9dd960899a95b))
* sync generated commands ([023855e](https://github.com/langchain-ai/langsmith-cli/commit/023855ef6372acec3481274807c17471b7afeda0))
* sync generated commands ([406f932](https://github.com/langchain-ai/langsmith-cli/commit/406f932eb39bf9725d5b8bb13df2bdffbf7736fb))
* sync generated commands ([6dc7784](https://github.com/langchain-ai/langsmith-cli/commit/6dc7784f7eba2f65924524e510c9ac2929d8031a))
* sync generated commands ([c312b37](https://github.com/langchain-ai/langsmith-cli/commit/c312b37c1f12cb70f191fb9433e2af07033370f5))


### Refactors

* **trace:** set the trace feedback window through typed SDK fields ([b1efca4](https://github.com/langchain-ai/langsmith-cli/commit/b1efca468bb950025275bd90990cb4dad7693035))
* **trace:** set the trace feedback window through typed SDK fields ([f1c4f02](https://github.com/langchain-ai/langsmith-cli/commit/f1c4f02736f3fa1623f8fc42a1504039ab4f0ea4))
