# Salesforce Headless 360 Trust Metadata

Deploy this metadata when an org edition cannot create Tooling API
`Certificate` records and must use the `SF360_Bundle_Key__mdt` fallback.

```sh
sf project deploy start --source-dir metadata --target-org <alias>
sf org assign permset --name SF360_Key_Registrar --target-org <alias>
```

After deployment, run:

```sh
salesforce-headless-360-pp-cli trust register --org <alias>
```

`trust register` prefers Certificate records. If Salesforce returns
`INVALID_TYPE` or `NOT_FOUND` for `Certificate`, it writes a CMDT key record
with a signed hash-chain receipt instead.
