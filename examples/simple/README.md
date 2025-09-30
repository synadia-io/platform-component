### Steps for connecting a platform component

These steps demonstrate connecting a platform component to a locally running Synadia Control Plane instance.
The steps connect a `workloads` platform component.
After connecting, the example publishes a single NATS messages and closes the connection.

---

1. Create workloads platform component for a system. This also creates a platform component token. If a config is not specified, the default control account is used and a default nexus name of "nexus" is used.

```shell
❯ SYSTEM_ID=33E4oTxAbbLsPmeFZBySaK2MaIC

# returns HTTP 204 if succesfful
❯ curl -X PATCH "$SCP_URL/api/core/beta/systems/$SYSTEM_ID/platform-components" \
 -H "Authorization: Bearer $SCP_API_TOKEN" \
 -H 'content-type: application/json' \
 -d '{"config":{},"enabled":true,"type":"workloads"}'
```

2. Get the platform component ID from the system response.
```shell
❯ curl -X GET "http://localhost:3000/api/core/beta/systems/$SYSTEM_ID" \
 -H "Authorization: Bearer $SCP_API_TOKEN" \
 -H 'accept: application/json' | jq -r '.platform_components.components | map(select(.type = "workloads")) | .[0].id'

33QczjmtKwvOxKwxIOSPKXBzhnC
```

3. Use the platform component ID to get the platform component token.

```shell
❯ PLATFORM_ID=33QVUSQNH3uE3LAWX7L7Zu1NEeb

❯ curl -X GET "http://localhost:3000/api/core/beta/systems/$SYSTEM_ID/platform-components/$PLATFORM_ID/tokens" \
 -H "Authorization: Bearer $SCP_API_TOKEN" \
 -H 'accept: application/json' | jq -r .token

pcm_...
```

4. Use the platform component token with the example service to connect and register with Control Plane.


```shell
❯ cat .env
SCP_URL=http://localhost:3000
SCP_PLATFORM_TOKEN=pcm_...  # value from previous step

❯ go run .
{"time":"2025-09-30T11:25:08.922258-06:00","level":"INFO","msg":"connecting to platform","server":"http://localhost:3000","user":"UBYZUHBTU4HPSKXLD5ILNIJWZCXRYU2VMRCWQABIBNRPWQVYVDMBOBCP"}
{"time":"2025-09-30T11:25:08.968557-06:00","level":"INFO","msg":"register request success!"}
{"time":"2025-09-30T11:25:08.968569-06:00","level":"WARN","msg":"control plane returned config data but no destination supplied"}
{"time":"2025-09-30T11:25:08.968573-06:00","level":"INFO","msg":"connecting to nats","server":"nats://localhost:14222"}
{"time":"2025-09-30T11:25:08.974985-06:00","level":"INFO","msg":"connected"}
2025/09/30 11:25:08 message published
{"time":"2025-09-30T11:25:08.975009-06:00","level":"INFO","msg":"stopping platform component"}
{"time":"2025-09-30T11:25:08.975034-06:00","level":"INFO","msg":"starting heartbeat"}
{"time":"2025-09-30T11:25:08.975099-06:00","level":"INFO","msg":"heartbeat stopped"}
```
