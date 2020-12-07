# revizor docker action

This action `create|delete` the test environemnt container.

To configure this action, you need to provide the following environment variables:
- `REVIZOR_URL`: Revizor base url.
- `REVIZOR_TOKEN`: Revizor access token.
- `SCALR_TOKEN`: Scalr access token.

## Inputs

### `command`

**Required** The command to execute: `create|delete`.

### `conatiner_id`

**Optional** The container ID.

## Outputs

### `conatiner_id`

The container ID.

### `hostname`

The test environment hostname.

## Example usage
```yaml
uses: Scalr/gh-action-revizor@master
with:
    command: create
```