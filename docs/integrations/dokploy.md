# Dokploy environments

LogDeck reads and writes Dokploy's saved configuration rather than recreating a managed container. Saving and deploying are separate actions. A successful save does not mean the running workload has changed; an accepted deployment request does not prove deployment completion.

## Connect an instance

In Settings → Connections, add the LogDeck Docker host name, Dokploy instance URL, API token (Dokploy calls it an API key), and Dokploy deployment server ID. Leave the server ID empty only for the server running that Dokploy instance. For an instance managing multiple hosts, add one connection per LogDeck Docker host, with the appropriate server ID. The URL may include `/api`. Use an `https://` URL where you can: over `http://` the API token is sent unencrypted, so keep that to a trusted private network.

Connections created in Settings persist in `/data/config.json`. Mount `/data` to retain them across container replacement. API tokens are masked in settings responses. Environment-defined entries cannot be changed through the UI.

Alternatively, set:

```text
DOKPLOY_CONFIGS=local|https://dokploy.example.com|key|,remote|https://dokploy.example.com|key|remote-server-id
```

Connection testing checks whether accessible resources can be discovered. It does not certify environment-write or deployment-create permissions. Those permissions are checked by Dokploy when each action is requested. A valid token can have no accessible projects; an empty inventory cannot establish that a managed container is safe to recreate.

## Confirm ownership and edit

Open the container's Environment panel. LogDeck reads `project.all`, then each accessible application's or Compose deployment's record to obtain its deployment name and server ID. Inventory summaries alone omit this information. It filters resources to the configured deployment server, suggests exact matches to container Compose/Swarm metadata, and requires confirmation of the selected resource.

The confirmed deployment is remembered in the browser for that container, so reopening the panel goes straight to the editor; **Change deployment** forgets it. Each action still verifies the resource ID, type, server, instance, and deployment name again. A renamed resource or changed instance/server requires a new confirmation. The user must connect the correct server for the Docker host; this is not an automatic verification of Docker endpoint identity. Swarm worker nodes must be associated with the Dokploy server that owns the deployment.

The editor works with raw environment text, retaining comments, quoted multiline values, and shared or vault references. Values are initially hidden. Editing changes service-level configuration, not shared project/environment variables or the Compose definition.

| Resource         | Save operation                | Settings retained                                                             | Apply operation                          |
| ---------------- | ----------------------------- | ----------------------------------------------------------------------------- | ---------------------------------------- |
| Application      | `application.saveEnvironment` | `buildArgs`, `buildSecrets`, `createEnvFile`, including nullable build fields | `application.deploy`                     |
| Compose or Stack | `compose.saveEnvironment`     | `createEnvFile`; no application build fields are sent                         | `compose.deploy`, without `freshVolumes` |

Compose variables belong to the whole deployment. They enter a service only when the Compose definition references them or uses `env_file`. Saving a variable does not add that reference. Deployment may restart several containers.

Known Dokploy workloads and Swarm tasks cannot use direct Docker recreation. For an ordinary Compose workload on a connected host, choose **Not managed by Dokploy** to use the runtime editor if Dokploy does not own it. The choice is remembered in the browser; **Map to Dokploy** undoes it. This option is unavailable when Dokploy-specific metadata or Swarm ownership is present. Runtime edits must also be persisted in the Compose source file to survive Compose recreation. Standalone containers retain the existing Docker editor.

Preview deployments, database resources, and project/environment-level variable editing are unsupported here. Use Dokploy for those operations. Recognized previews are blocked from being mapped to production applications. CLI and MCP environment edits do not support the confirmed Dokploy mapping workflow; use the UI.

## Failures and concurrent edits

Missing credentials, inaccessible resources, denied permissions, missing required fields, and unsupported settings produce errors instead of falling back to Docker recreation. When Dokploy cannot be read at all, a Compose workload with no Dokploy or Swarm metadata still offers **Not managed by Dokploy**, since the runtime editor never depended on that inventory. Save failures and uncertain deployment requests keep the draft editable but require reloading the saved configuration and checking Dokploy before retrying. LogDeck does not automatically retry deployment.

Saves compare the loaded revision with a fresh copy of every field the save API overwrites. A mismatch rejects the edit. Writes from LogDeck are serialized per instance and resource, including replicas and Compose services that share a record. Dokploy does not expose a conditional-save contract: another external writer can still change configuration between LogDeck's check and Dokploy's write. This is stale-edit detection, not an atomic concurrency guarantee.

## Documentation and source checked

The implementation was checked against these Dokploy references:

- [REST API and authentication](https://docs.dokploy.com/docs/api)
- [Application API](https://docs.dokploy.com/docs/api/application)
- [Compose API](https://docs.dokploy.com/docs/api/compose)
- [Project inventory API](https://docs.dokploy.com/docs/api/project)
- [Server API](https://docs.dokploy.com/docs/api/reference-server)
- [Applications and multiline environment values](https://docs.dokploy.com/docs/core/applications)
- [Compose environment injection and Stack behavior](https://docs.dokploy.com/docs/core/docker-compose)
- [Shared variables and vault references](https://docs.dokploy.com/docs/core/variables)
- [Remote deployment servers](https://docs.dokploy.com/docs/core/remote-servers)

The API examples omit useful response schemas. Router, database-schema, deployment-builder, permission, and preview code were inspected at [Dokploy commit 48504fde](https://github.com/Dokploy/dokploy/tree/48504fde4eb210056f7d9f80406f9692a1a7ea8a). This clarified required application save fields, nullable build values, inventory nesting, permission filtering, preview deployment names, and the absence of atomic save revisions.

Automated coverage uses HTTP contract fixtures and UI interactions. A live Dokploy instance has not been exercised in this workspace. Validate against the installed Dokploy version before treating live deployment behavior as verified.
