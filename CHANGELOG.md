# Changelog

All notable changes to LogDeck are documented here.

## [0.11.0](https://github.com/AmoabaKelvin/logdeck/compare/v0.10.0...v0.11.0) (2026-10-31)


### Features

* **alerts:** add Slack, Discord, email, PagerDuty and Opsgenie channels ([974634a](https://github.com/AmoabaKelvin/logdeck/commit/974634aad84261303e72b3d0faaa5f9d59f87f4b))
* **alerts:** add Slack, Discord, email, PagerDuty and Opsgenie channels ([8b25c41](https://github.com/AmoabaKelvin/logdeck/commit/8b25c4174ee3045e5260b5bda3db55b9e93d5ce8))
* **alerts:** alert on sustained CPU/memory use and restart loops ([0be9558](https://github.com/AmoabaKelvin/logdeck/commit/0be955899caadd67fdefa6529415679b5ca9075e))
* **alerts:** alert on sustained CPU/memory use and restart loops ([020d884](https://github.com/AmoabaKelvin/logdeck/commit/020d88408c65a0a1f8f9ac0b09ee425525b3c78a))
* **alerts:** silence alerts and schedule maintenance windows ([134c5f1](https://github.com/AmoabaKelvin/logdeck/commit/134c5f1a0ceb25daeebd17fca27525873d8bae75))
* **alerts:** silence alerts and schedule maintenance windows ([aecac1f](https://github.com/AmoabaKelvin/logdeck/commit/aecac1fce293707136442a6c5bc6025bafa747c5))
* **audit:** name the silence and image update actions ([14b4b55](https://github.com/AmoabaKelvin/logdeck/commit/14b4b55c8281e548e02d8f3c1977f580f2760787))
* **audit:** record container and settings changes in an audit log ([24a07d2](https://github.com/AmoabaKelvin/logdeck/commit/24a07d222c6cc2096d2fbb925656cb2af065c6ea))
* **audit:** record container and settings changes in an audit log ([96827d4](https://github.com/AmoabaKelvin/logdeck/commit/96827d4b1a150fc3535f8ab1df54440b4dfef909))
* **auth:** add multiple users with roles and OIDC sign-in ([e65ff98](https://github.com/AmoabaKelvin/logdeck/commit/e65ff9864c5a425a94be9f06f82253ae28a165af))
* **auth:** add multiple users with roles and OIDC sign-in ([c65a39c](https://github.com/AmoabaKelvin/logdeck/commit/c65a39c86be7c568bcb90e0980dd95a6452974c0))
* **cli:** page through the audit log with --offset ([b85d5fd](https://github.com/AmoabaKelvin/logdeck/commit/b85d5fd4285ecf95d2cf15d588dd8c99437f3f2c))
* **images:** show image updates and update containers from LogDeck ([6938a29](https://github.com/AmoabaKelvin/logdeck/commit/6938a295fb1e2f7848b014d1b22c502314c10748))
* **images:** show image updates and update containers from LogDeck ([7033361](https://github.com/AmoabaKelvin/logdeck/commit/7033361c9331c1e9bd1cc54fc0280c70aa0d3c68))
* **logs:** filter structured log lines by field ([91de0b8](https://github.com/AmoabaKelvin/logdeck/commit/91de0b83bd475819270bb3d08fda68ff863f7ea4))
* **logs:** search logs by structured fields ([f793cc9](https://github.com/AmoabaKelvin/logdeck/commit/f793cc937e00c272a7293574da5c0bc7d89f5c99))


### Bug Fixes

* **alerts:** widen the maintenance window start time input ([15143b3](https://github.com/AmoabaKelvin/logdeck/commit/15143b39b6c25ba29c210f723d9bd03997db6b4a))
* **alerts:** widen the resource rule metric select ([c6a7fff](https://github.com/AmoabaKelvin/logdeck/commit/c6a7fff7cbb6a70e738e2ce82a8fd13798a5315c))
* **alerts:** wrap long rule targets and triggers instead of scrolling ([d9eda42](https://github.com/AmoabaKelvin/logdeck/commit/d9eda42f78a10231eb555ede5c5ccd77d66ba9b9))
* **audit:** clamp the page only on fresh results ([498993c](https://github.com/AmoabaKelvin/logdeck/commit/498993cd26055f9a0c787466e6be2dc531a9b384))
* **audit:** drop the every-change claim from the settings copy ([d9e24e5](https://github.com/AmoabaKelvin/logdeck/commit/d9e24e50aad569d76e239a12fb43242bb9206ba8))
* **audit:** keep entries across oversized and unterminated lines ([57d0861](https://github.com/AmoabaKelvin/logdeck/commit/57d0861f7d1553af52377714de2bb5c143d31ec8))
* **audit:** return to a valid page when the results shrink ([bf5cf73](https://github.com/AmoabaKelvin/logdeck/commit/bf5cf73c065ade61fbaad5bf58ee954cc992b9cd))
* **audit:** shorten filter placeholders so they fit ([33ca8d3](https://github.com/AmoabaKelvin/logdeck/commit/33ca8d39e1cf312f5d4dfe090a54c685a688fe69))
* **audit:** start a new line after a failed write ([439512c](https://github.com/AmoabaKelvin/logdeck/commit/439512c4c2dd0fbd18fbe8834587a99e8206cbdd))
* **auth:** keep a legacy admin in the config so a rollback still requires login ([be3565d](https://github.com/AmoabaKelvin/logdeck/commit/be3565d64394f600aaa29dcc202ff26f6c078f98))
* **auth:** keep env auth closed to the file's old admin and bind SSO tokens to the tab ([2f1ea31](https://github.com/AmoabaKelvin/logdeck/commit/2f1ea31cc89f81bcc680bdf02f6d21ab7378c85b))
* **auth:** name user changes in the audit log and disable actions for read-only users ([1414899](https://github.com/AmoabaKelvin/logdeck/commit/14148992709f340a6da11bbe683877ab51cfef38))
* **oidc:** tidy the SSO settings form and login spacing ([16d589e](https://github.com/AmoabaKelvin/logdeck/commit/16d589e735b856cefd84c09c27edd3e711eac940))

## [0.10.0](https://github.com/AmoabaKelvin/logdeck/compare/v0.9.1...v0.10.0) (2026-10-30)


### Features

* **backup:** add backup and restore to Settings ([5e8853c](https://github.com/AmoabaKelvin/logdeck/commit/5e8853c24e15c7151607abe25dd1d29e25240123))
* **backup:** add backup and restore to Settings ([0f0b5ff](https://github.com/AmoabaKelvin/logdeck/commit/0f0b5ff32e91839c2d526a465c0a957fa05440c2))
* **containers:** say which platform recreates a managed container ([59e8bba](https://github.com/AmoabaKelvin/logdeck/commit/59e8bba34d4f60fe033c6f902ab94d57ad24c17c))
* **containers:** say which platform recreates a managed container ([0d8ec38](https://github.com/AmoabaKelvin/logdeck/commit/0d8ec383df85ec32a6cf2c0516e00be0d9b82362))
* **env:** show what is not deployed yet and confirm saves ([7febb37](https://github.com/AmoabaKelvin/logdeck/commit/7febb37ace4df10d7c3b170207f5c49efe49207c))
* **env:** show what is not deployed yet and confirm saves ([610d30e](https://github.com/AmoabaKelvin/logdeck/commit/610d30ead8afecb23899e7162c28fc3a4c6c9562))
* **health:** show collection health and alert when collection fails ([640da63](https://github.com/AmoabaKelvin/logdeck/commit/640da63b95248a26b2e1cc847887d6fa24d0198f))
* **health:** show collection health and alert when collection fails ([a74fd2d](https://github.com/AmoabaKelvin/logdeck/commit/a74fd2d6b8a77873a7b40d07a57f9a1dbca409c5))
* **redaction:** mask secrets in logs before storage and delivery ([7dadebc](https://github.com/AmoabaKelvin/logdeck/commit/7dadebcbaf3746c09a732993cfbb56deca11bdc1))
* **redaction:** mask secrets in logs before storage and delivery ([4c0b700](https://github.com/AmoabaKelvin/logdeck/commit/4c0b7009dd0fbc1b93162177c736223083ffd1c1)), closes [#168](https://github.com/AmoabaKelvin/logdeck/issues/168)
* **resources:** save managed-container limits in Coolify and Dokploy ([7298b99](https://github.com/AmoabaKelvin/logdeck/commit/7298b99b0f0429bed12d5a8dfa696bc4ea33f64a))
* **resources:** save managed-container limits in Coolify and Dokploy ([452c82c](https://github.com/AmoabaKelvin/logdeck/commit/452c82c6c76bec8d1d2d6c7c2a15d4af74497201))


### Bug Fixes

* **alerts:** use the existing alert triangle icon ([9af28fb](https://github.com/AmoabaKelvin/logdeck/commit/9af28fbd50dcf100674eb38fd51ad185589b9607))
* **logs:** report omitted sources and partial failures in combined logs ([bd92fca](https://github.com/AmoabaKelvin/logdeck/commit/bd92fcab967d462575e59771f365686b292c737a))
* **logs:** report omitted sources and partial failures in combined logs ([b285f6a](https://github.com/AmoabaKelvin/logdeck/commit/b285f6ab5b5d83719103a77ef2a7d02cc4a9edcf))

## [0.9.1](https://github.com/AmoabaKelvin/logdeck/compare/v0.9.0...v0.9.1) (2026-10-17)


### Bug Fixes

* **dokploy:** check hosts side by side, keep earlier confirmations ([b0da365](https://github.com/AmoabaKelvin/logdeck/commit/b0da365a7d698c42baf2cd70087dabf74b32fc30))
* **dokploy:** recognize Dokploy's Swarm services ([c3503be](https://github.com/AmoabaKelvin/logdeck/commit/c3503bee607d0db8f5a2cef6b4c26ba330b81aa2))
* **dokploy:** recognize Dokploy's Swarm services ([1ac28f3](https://github.com/AmoabaKelvin/logdeck/commit/1ac28f35bca3fe0de5e2e01a7fc2134466011cfb))


### Performance

* **dokploy:** read records in parallel, wait on the first refresh only ([e6a3e37](https://github.com/AmoabaKelvin/logdeck/commit/e6a3e3706ca483506707dc14424222461f4da47c))

## [0.9.0](https://github.com/AmoabaKelvin/logdeck/compare/v0.8.3...v0.9.0) (2026-10-05)


### Features

* **dokploy:** edit saved environments for Dokploy deployments ([b833ab2](https://github.com/AmoabaKelvin/logdeck/commit/b833ab2154af5f555b7e2ab3ee77868faa1aa9ab))
* **dokploy:** edit saved environments for Dokploy deployments ([30b8060](https://github.com/AmoabaKelvin/logdeck/commit/30b80604bdc81593d25ad239a9017683e97c887e)), closes [#165](https://github.com/AmoabaKelvin/logdeck/issues/165)


### Bug Fixes

* **coolify:** preserve secrets when replacements are cleared ([84ee55c](https://github.com/AmoabaKelvin/logdeck/commit/84ee55c9fd781bc13e6fd895bc7977b1df5e9315))
* **coolify:** save environment changes before deploying ([474470b](https://github.com/AmoabaKelvin/logdeck/commit/474470bc2b50a37f0bb8996d6c32f8949d49fed6))
* **coolify:** save environment changes before deploying ([24178a8](https://github.com/AmoabaKelvin/logdeck/commit/24178a8b86a17560c516f472141a141b8a6b41f9)), closes [#164](https://github.com/AmoabaKelvin/logdeck/issues/164)
* **dokploy:** open the dashboard, not /api, from the env panel ([5d6b1a0](https://github.com/AmoabaKelvin/logdeck/commit/5d6b1a0cb9d72f6f3eba8bd610718929947960f0))
* **dokploy:** use status constants and lowercase client errors ([f6ee33b](https://github.com/AmoabaKelvin/logdeck/commit/f6ee33bb8f7625f6a14dc8c3bed1f3f57e178397))

## [0.8.3](https://github.com/AmoabaKelvin/logdeck/compare/v0.8.2...v0.8.3) (2026-09-28)


### Bug Fixes

* **auth:** clear cached data on logout ([b4a47ad](https://github.com/AmoabaKelvin/logdeck/commit/b4a47ad34ba4b61202b3220fb710cc4656cd4ece))
* **auth:** reject backslashes in the login redirect ([02e3a14](https://github.com/AmoabaKelvin/logdeck/commit/02e3a145bf54bc7d9dbc0a9119ddbb43e29cf3ac))
* **auth:** return to the same page after signing in ([7220fdd](https://github.com/AmoabaKelvin/logdeck/commit/7220fdd3ad326a01bbcb1ed0fc35ae3e8f99a277))
* **containers:** only say not found after the store answers ([7ab4b2f](https://github.com/AmoabaKelvin/logdeck/commit/7ab4b2f3e12b80cdb65ab8e8a1ed41e90347ba81))
* **containers:** say when a container can't be found ([55b12aa](https://github.com/AmoabaKelvin/logdeck/commit/55b12aac6509b3ed4dad6c5e661bf1a8123520ce))
* **docker:** stop waiting on hosts that don't answer ([4662b05](https://github.com/AmoabaKelvin/logdeck/commit/4662b055bcf46192ddc987b937ad425637150be8))
* **logs:** arrow keys scroll the page unless the log list has focus ([8f54f46](https://github.com/AmoabaKelvin/logdeck/commit/8f54f4614c9035532d6c14f9f8a09ac2d00d4fc5))
* **logs:** refresh moves relative time ranges to now ([4304733](https://github.com/AmoabaKelvin/logdeck/commit/43047333da51e2c21bd9da0f821a8d76432052c1))
* slow hosts, stack streams and smaller UI bugs ([c55c176](https://github.com/AmoabaKelvin/logdeck/commit/c55c176370c8244d975f20e50e77b61518b9252a))
* **stacks:** restart the merged stream when a member starts ([f0d9854](https://github.com/AmoabaKelvin/logdeck/commit/f0d9854436a582a907b0d8a233a514db6c2b7e48))
* **stacks:** restart the stream on a quick member restart ([218aa28](https://github.com/AmoabaKelvin/logdeck/commit/218aa28ed535fa001e0fb65038df8f67812c3b76))

## [0.8.2](https://github.com/AmoabaKelvin/logdeck/compare/v0.8.1...v0.8.2) (2026-09-26)


### Bug Fixes

* **alerts:** don't fire died alerts on stops and restarts ([9480a2e](https://github.com/AmoabaKelvin/logdeck/commit/9480a2e0ff963a63acab5b5b9dea8242f269e94b))
* **api:** bound the login body and the hostname lookup ([8a3b91a](https://github.com/AmoabaKelvin/logdeck/commit/8a3b91a1d0e04d7566aa5ce31e44f270bb4a9db9))
* **api:** hide container env from read tokens ([07c880f](https://github.com/AmoabaKelvin/logdeck/commit/07c880faf470e904a0c90744acfae72262904fb8))
* **api:** reopen the events stream after a host change ([1a860a8](https://github.com/AmoabaKelvin/logdeck/commit/1a860a8caee226e6637cb6dba93f6dd1c25a95b7))
* **auth:** sign out sessions when the password changes ([25e9294](https://github.com/AmoabaKelvin/logdeck/commit/25e9294eb75250818cc266a61b9e1e239f9368f6))
* backend bugs from the review pass ([4723138](https://github.com/AmoabaKelvin/logdeck/commit/472313844b777f9d8d914e620fb63bcb91fbf320))
* **dashboard:** keep the logs sheet in sync with the container ([e0dc5ea](https://github.com/AmoabaKelvin/logdeck/commit/e0dc5ea57d8571d5d2387bf3d8bcf2ca410f027f))
* **docker:** forget the stop mark when a stop fails ([dc17d30](https://github.com/AmoabaKelvin/logdeck/commit/dc17d30ffc4df9adc4b62e3024c237a742271088))
* **docker:** keep anonymous volumes when editing env ([fbabb25](https://github.com/AmoabaKelvin/logdeck/commit/fbabb25b3c28ea8df6a9ab3e73e4c5361914d4e8))
* **env:** show remove on any touch screen ([f5a2e51](https://github.com/AmoabaKelvin/logdeck/commit/f5a2e51d682c61f07001e2abc04e4624ff8a1492))
* **exec:** report truncated output and the real exit code ([77387dd](https://github.com/AmoabaKelvin/logdeck/commit/77387dd84ad80a960ee7284b5243e42892e8d944))
* frontend bugs from the review pass ([79cd8c7](https://github.com/AmoabaKelvin/logdeck/commit/79cd8c7f8638340215bafbf55dc5d35d2f010200))
* **frontend:** keep the session on server errors, show env errors ([6f8dc64](https://github.com/AmoabaKelvin/logdeck/commit/6f8dc64d154da9e7e704d4d4c36f5133e8580906))
* **frontend:** label icon buttons, keep loaded data on refetch errors ([66963a3](https://github.com/AmoabaKelvin/logdeck/commit/66963a381e380f5f153af043adae139bbd2687e4))
* **logs:** drop log fetches that resolve after a newer one ([7cbddb7](https://github.com/AmoabaKelvin/logdeck/commit/7cbddb7d3a1dbc90e3e5fff40d3b24e3970e02a3))
* **logs:** read TTY containers' live logs ([a4bb082](https://github.com/AmoabaKelvin/logdeck/commit/a4bb08222a4081170a67cc9179ff9af52882886f))
* **logstream:** stop tails for removed hosts ([1bf24dc](https://github.com/AmoabaKelvin/logdeck/commit/1bf24dcf728309aff6ddbb277a1896efe639f792))
* **server:** end open streams on shutdown ([87ba1a0](https://github.com/AmoabaKelvin/logdeck/commit/87ba1a03c0e75813f9a4621ca4621571a5ecfb19))
* **settings:** respect read-only mode and refresh containers ([33cfac6](https://github.com/AmoabaKelvin/logdeck/commit/33cfac6ec29c0636615b850dc985869541ee5b8e))
* **stats:** fetch each host's stats as soon as it lists ([6aea8de](https://github.com/AmoabaKelvin/logdeck/commit/6aea8de60376ac074978a9b3caa9cfd180f931d2))
* **stats:** list hosts in parallel ([6d1d4cb](https://github.com/AmoabaKelvin/logdeck/commit/6d1d4cbb17bff0a251ea66db84d9a21732e7b274))

## [0.8.1](https://github.com/AmoabaKelvin/logdeck/compare/v0.8.0...v0.8.1) (2026-09-25)


### Bug Fixes

* **logstore:** advance both watermarks after a full read ([56217ab](https://github.com/AmoabaKelvin/logdeck/commit/56217ab65cea1d108956fedabdb90b8ce3b18af9))
* **logstore:** advance both watermarks after a full read ([6d27190](https://github.com/AmoabaKelvin/logdeck/commit/6d2719023de1aeb1ec9b069bba2fe7a12d8086b1)), closes [#155](https://github.com/AmoabaKelvin/logdeck/issues/155)

## [0.8.0](https://github.com/AmoabaKelvin/logdeck/compare/v0.7.0...v0.8.0) (2026-09-24)


### Features

* **podman:** group Quadlet containers by pod or stack label ([0f5dc70](https://github.com/AmoabaKelvin/logdeck/commit/0f5dc705ed961d231fab9354e5756d350ccaf65a))
* **podman:** group Quadlet containers by pod or stack label ([b39011d](https://github.com/AmoabaKelvin/logdeck/commit/b39011dba4399e2c02b47d699d352eb5d263dd70)), closes [#124](https://github.com/AmoabaKelvin/logdeck/issues/124)


### Bug Fixes

* **podman:** address review on Quadlet grouping ([0cb32a6](https://github.com/AmoabaKelvin/logdeck/commit/0cb32a6801eeb4256d906ac8d360683c81948f70))

## [0.7.0](https://github.com/AmoabaKelvin/logdeck/compare/v0.6.0...v0.7.0) (2026-09-23)


### Features

* **history:** search stored logs across containers and stacks ([fad7415](https://github.com/AmoabaKelvin/logdeck/commit/fad7415d225268ce82a86e35d82dd6c4633de368))
* **history:** search stored logs across containers and stacks ([2234c1f](https://github.com/AmoabaKelvin/logdeck/commit/2234c1f677543ac2aef4b5e4c14f05ed9bafc3a7))


### Performance

* **models:** skip level and grouping regexes that cannot match ([d91e7cc](https://github.com/AmoabaKelvin/logdeck/commit/d91e7cc32bb41ef0a455f1ec8f34b1844d000bc8))

## [0.6.0](https://github.com/AmoabaKelvin/logdeck/compare/v0.5.0...v0.6.0) (2026-09-23)


### Features

* **dashboard:** history toggle in the logs sheet ([e3df945](https://github.com/AmoabaKelvin/logdeck/commit/e3df945cbd72a9b734bc6554fb8d5d9b377b9b29))
* **dashboard:** history toggle in the logs sheet ([72d8256](https://github.com/AmoabaKelvin/logdeck/commit/72d825605065f97a1d658c3bedc2bae305fe1098))
* **history:** add a delete-all for stored logs ([3f36964](https://github.com/AmoabaKelvin/logdeck/commit/3f3696476877cee86ce5b60ea82510cbb776f660))
* **history:** page and search stored containers server-side ([03ac0fc](https://github.com/AmoabaKelvin/logdeck/commit/03ac0fc80814aa05bb641fb0ccad850c00440875))
* **history:** server-side paging and delete-all for stored logs ([1dbb9d5](https://github.com/AmoabaKelvin/logdeck/commit/1dbb9d5062bcb325bcbc44f65057ce461ca86ac0))
* **logstore:** expire removed containers' logs after 30 days ([c2c565b](https://github.com/AmoabaKelvin/logdeck/commit/c2c565b5ee55fa514ec02dc4b942fd39df6622d5))
* **logstore:** expire removed containers' logs after 30 days ([5292142](https://github.com/AmoabaKelvin/logdeck/commit/52921429db1cb91b4c69c83dd80e5b99352d74ad))
* **settings:** filter stored containers, delete all removed at once ([23d3239](https://github.com/AmoabaKelvin/logdeck/commit/23d32399fc56b95f78e21f482d6932ee5597b9a8))
* **settings:** filter stored containers, delete all removed at once ([999370b](https://github.com/AmoabaKelvin/logdeck/commit/999370bf241cf50498fdfccbc926fa23368e058b))


### Bug Fixes

* **frontend:** run the unit tests again, and in CI ([d7bb912](https://github.com/AmoabaKelvin/logdeck/commit/d7bb9122be056102966877d6c259f37f55a6e0ca))
* **frontend:** run the unit tests again, and in CI ([8e6efbc](https://github.com/AmoabaKelvin/logdeck/commit/8e6efbcb727f420c9a795faa701b5687909badcd))

## [0.5.0] - 2026-09-22

### Added

**Settings page.** Settings uses the same layout as the containers and container pages: full width, ruled sections instead of cards, and a side nav in place of the tab pills. Inline forms for adding a host, token or channel sit in recessed wells and stack on phones. Log storage shows used space, the per-container cap and the stored container count as a readings strip, and the stored containers table is paginated, 10 per page. It used to render every row, which froze the page once the store held a few hundred containers.

**Logs sheet.** Opening logs from the dashboard shows a slice of the container page: the name and state in the title, the same CPU, memory, uptime and ports strip, and the same Overview, Network, Environment and Limits panels. The live stream fills the rest of the sheet instead of a fixed 400px box. The old details card, label list and environment toggle are gone, since the panels cover them. Open page links to the full container page.

### Changed

- The dashboard opens on running containers. With log history kept, exited and removed containers were crowding out the default view. All states is one click away in the state filter.
- Redis logs get a level from Redis's single-character marker, and Postgres DETAIL, HINT and STATEMENT lines fold into the ERROR or LOG line above them. Both used to show as separate UNKNOWN rows.
- Multi-line stack traces stay in one row under the right level. A Go panic used to split into seven rows and a Python traceback into three, and an exception printed after an INFO line was folded into that INFO entry, so an ERROR filter never showed it. Exception class names like `TypeError` count as ERROR, phrases like `0 failed`, `err=nil` and `error handler` no longer count as a level, and access log lines take their level from the status code (5xx ERROR, 4xx WARN).
- The README is a quick start with links; the full feature tour lives on logdeck.dev.

### Fixed

- A burst of parallel requests to an SSH host spawned an ssh process per request before the master socket was ready, which tripped sshd's MaxStartups and dropped connections. Dials to a host now go one at a time until the connection is up.
- Replaying a batch of interleaved containers into the log store decompressed the same block once per line switch. Decoded blocks are now cached across a commit, capped at 16 MB.

## [0.4.0] - 2026-09-17

### Added

**Sortable container table.** Every column header sorts: name, host, status, uptime, created, ports, and usage. Usage sorts by CPU or memory, picked from a small menu beside the header, and re-sorts as new stats arrive. Containers with nothing to sort on, such as stopped ones under CPU, stay at the bottom in both directions. The sort is part of the URL, so a bookmarked or shared link opens the same view.

**Host column and a Columns menu.** The table can show which host each container runs on. The column is off by default. A Columns menu in the toolbar shows or hides every column except the name and the row actions, and the choice is remembered in the browser.

**Collapsible project groups.** With grouping on, each compose project has a header with a chevron, a container count, and how many are running. Its containers are indented under it. Click a header to fold the project away, or use Collapse all and Expand all in the grouping menu. Collapsed projects are remembered in the browser.

**Fullscreen logs and text size.** The log viewer on the container and stack pages can fill the screen from a toolbar button (`f` toggles, `Esc` leaves). The stream, scroll position, and selection carry over, and `?fullscreen=true` in the URL reopens it after a refresh. Text size is in the View menu, 11 to 20px, and `+` and `-` also work.

### Changed

- Grouping by project covers the whole filtered list, and pagination is hidden while it is on. Groups used to be built one page at a time, so a project could be split across pages and show the wrong count.

### Fixed

- With the same container name on two hosts, the container page always opened the first host's container. The header named the wrong host, and the logs and actions went to the wrong container as well. Links to the page now carry the host. Old links without one behave as before.
- The dashboard read "LogDeck on logdeck", or a container ID, when LogDeck ran in a container, because the hostname it reported was the container's own. It now reports the machine's name, taken from the local Docker or Podman engine.
- The toolbar above the container table overflowed the screen on phones. Its controls wrap now.
- A browser with localStorage blocked or full could fail to load the dashboard. Table preferences that cannot be saved now last for the session.

## [0.3.1] - 2026-09-16

### Fixed

- Every followed log, stats, and event stream over an SSH host died after ten seconds and reconnected through a new `ssh` process, and each stats poll opened one more process per container. Remote sshd throttled the handshakes. Streams now stay open, and every connection to a host is multiplexed over one OpenSSH ControlMaster session, so the remote sees a single login.
- LogDeck could sit at 140% CPU logging `database is locked`. Container metadata updates competed with log ingestion for SQLite's write lock during heavy backfills, and a failed commit was retried at full speed, which re-read and dropped the same batch in a tight loop. Writes are serialized on one connection now, and the writer backs off 1s to 5s after a failed commit.
- Two admins or agents saving the Docker or Coolify host list at the same time silently lost one of the changes. The settings response carries a revision per host list and a stale write is rejected with 409. The MCP host tools require it; the HTTP endpoint accepts it but does not insist, so older clients keep working.

## [0.3.0] - 2026-09-15

### Added

**MCP server.** `logdeck mcp` runs a Model Context Protocol server over stdio, so an AI assistant can list containers, read live and stored logs, check stats and history, edit environment variables and settings, and start, stop, restart, remove, or run commands in containers through the same API the CLI uses. Every tool is registered. The API token you hand it decides what works. A read-scoped token gets a read-only assistant, an admin token gets everything, and the server checks the scope on each call. See the MCP server docs page for client setup.

**Notification channels.** The single alert webhook is now a list of channels: generic webhook (Slack and Discord accept it as-is), ntfy, Gotify, and Telegram. Each alert goes to every enabled channel. Every channel has a test button in Settings and `logdeck alerts channels test`. An existing webhook config becomes a webhook channel on upgrade.

**Unhealthy alerts.** Event rules can fire when a container's healthcheck goes unhealthy, next to died and OOM-killed, with a "Health check failing" preset. Recoveries do not fire. Works on Podman, which leaves the state out of the event.

**Container detail page.** It opens with the container itself: name, state, image, host, live CPU, memory, and uptime, with the lifecycle actions in the header and the log stream taking the rest of the page. Panels below show what Docker's inspect payload always carried but the UI never read: restart count, exit code, OOM state, the healthcheck probe and its last five results with output, published ports as links, networks with addresses, mounts, a filterable environment editor that masks credential-looking values, and memory and CPU limits drawn against live usage. The terminal is a full-height dialog.

**Multi-line log events.** Stack traces, progress bars, and other spill-over lines fold into the entry they belong to instead of showing as separate rows with their own timestamps. The grouping recognises Python, Java, and Go tracebacks, and a progress bar rewritten with carriage returns shows its final state. Live streams fold on the server too, so live and history agree.

**Published ports** in the container list and the API.

**`PPROF_ADDR`** exposes Go heap and goroutine profiles on a loopback address. Off unless set.

### Changed

- Dashboard redesign. Navigation and global actions moved into a header, filters share one toolbar row, and status has its own column. Memory shows as an absolute against its limit (`52 MB of 64 MB`) instead of a percentage of an unstated ceiling, amber past 75% and rose past 90%. Long sha256 image references show in the short form Docker prints.
- Stored log history is compressed. Once a container has a thousand stored lines, the store seals them into one zstd block. On real Docker output that holds 8 to 16 times more history under the same retention cap, and full-history search reads far fewer pages. Lines stay byte-for-byte what the engine sent. The database file does not shrink, since SQLite reuses freed pages, but the caps now bound the compressed size.
- Log parsing costs a fraction of what it did. A line used to spend about 400 allocations on failed timestamp parses and seven regex passes on level detection. It now takes 3 allocations, and a line with no level keyword skips the regexes. A backfill that allocated 8 GB now allocates 0.8 GB, and memory after startup is about a third lower.
- The Docker image ships a stripped binary, 22 MB down to 16 MB.
- Go 1.25.

### Fixed

- Retention could not keep up under sustained writes. Its separate transaction lost the SQLite write lock to ingestion, nothing was evicted, and the file grew past the cap. Eviction now runs on the writer and the file stays near the cap under load.
- Stored logs had silent gaps. A container that stopped could lose lines Docker still had buffered, a burst that overflowed the live buffer dropped its oldest lines without repair, and a container that logged between its first backfill and the live stream attaching could lose that window. All three are detected now and re-read from the engine, with duplicates dropped.
- LogDeck over-reported container memory on cgroup v2 hosts, the modern default, by counting page cache as used. It now matches `docker stats`.
- Editing environment variables on Podman failed with `NanoCpus conflicts with CpuPeriod and CpuQuota` for any container with a CPU limit, from the web UI as well.
- Removing an environment variable you had just added still counted as a change and offered to recreate the container.

### Upgrading

The log database migrates itself. After the upgrade, every stored container is re-read once from the engine so the first-backfill fix takes effect. Inserts are deduplicated, so nothing is stored twice. Expect one burst of engine reads on first start.

An existing alert webhook becomes a webhook channel automatically. `logdeck alerts channels` manages the list.

## [0.2.0] - 2026-07-14

### Added

**Log persistence and history.** Container logs are stored on disk, so they stay readable after a container is restarted, rebuilt, or removed. A rebuilt container keeps one continuous timeline under its name, even though the engine gives it a new ID. A `Live | History` toggle in the log viewer pages back through stored logs with server-side level, regex, and time-range filtering. Containers that no longer exist appear on the dashboard under a "Removed" filter, and their logs remain readable until you delete them. Retention defaults to 50 MB per container and 1 GB total, editable in Settings.

**Alerting.** Rules fire on container events (died, OOM-killed) or on log output (level threshold plus an optional regex). A rate condition ("5 in 60 seconds") ignores noise, and a per-rule cooldown means a crash-looping container sends one alert instead of hundreds — repeat occurrences are counted and reported on the next alert. Alerts go to a JSON webhook that Slack, Discord, and ntfy accept as-is, and every alert is recorded in a history you can browse. Manage rules from Settings or the `logdeck alerts` CLI.

**Scoped API tokens.** Tokens are now `admin` or `read`. A read token can fetch logs, stats, events, and container details, but cannot mutate anything, use the web terminal, or read container environment variables, settings, or alert configuration — safe to hand to a CI job or an AI agent.

**Container health.** Healthy, unhealthy, and starting badges from Docker healthchecks, in the containers table and on the container page. (Podman's Docker-compatible list API does not report health, so badges there are Docker-only.)

### Changed

- Settings is organized into tabs: Connections, Access, Alerts, and Log storage. Each tab is linkable (`/settings?tab=alerts`).
- The `logdeck` CLI gains `alerts` (rules, history, webhook, test).

### Fixed

- `DOCKER_HOST` silently overrode every configured non-SSH host, collapsing a multi-host setup onto a single socket. An explicitly configured host now always wins.
- Read-scoped tokens could read the alert webhook URL, which is a secret.
- `env.example` advertised `BACKEND_PORT` and `FRONTEND_PORT`, which the server never reads, and omitted `CONFIG_PATH`, `READONLY_MODE`, and the retention caps.
- Documentation examples omitted the `/data` volume, so following them cost you your configuration on every upgrade.

### Upgrading

**Mount a volume at `/data`.** It holds the config file (Docker hosts, API tokens, alert rules), the stored logs, and the alert history. Without it, all of that is lost every time the container is recreated. LogDeck now warns at startup if the directory is not a mounted volume.

```
-v logdeck-data:/data
```

Log persistence is enabled by default. To turn it off, set `LOG_STORE_ENABLED=false`.

If you script the CLI with a read-scoped token, note that `logdeck alerts` now requires an admin token.

## [0.1.0] - 2026-07-10

First release. Multi-host Docker and Podman support, live and historical log viewing with level classification and search, compose stack grouping and actions, container lifecycle actions, environment and resource editing, a web terminal, image/volume/network views, host and container stats, read-only mode, authentication, API tokens, and the `logdeck` CLI.

[0.3.0]: https://github.com/AmoabaKelvin/logdeck/releases/tag/v0.3.0
[0.2.0]: https://github.com/AmoabaKelvin/logdeck/releases/tag/v0.2.0
[0.1.0]: https://github.com/AmoabaKelvin/logdeck/releases/tag/v0.1.0
