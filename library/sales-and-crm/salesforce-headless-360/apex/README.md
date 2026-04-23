# SF360 Safe Read Apex

Deploy from this directory with:

```sh
sf project deploy start --target-org <alias>
```

The class exposes `POST /services/apexrest/sf360/v1/safeRead` and executes SELECT
SOQL with `WITH USER_MODE` so Salesforce enforces the current user's sharing,
CRUD, and FLS before records leave the org.
