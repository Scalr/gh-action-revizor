# revizor docker action

This action `create|delete` the test environment container.

To configure this action, you need to provide the following environment variables:
- `REVIZOR_URL`: Revizor base url.
- `REVIZOR_TOKEN`: Revizor access token.
- `SCALR_TOKEN`: Scalr access token.

## Inputs

### `command`

**Required** The command to execute: `create|delete`.

### `container_id`

**Optional** The container ID.

## Outputs

### `container_id`

The container ID.

### `hostname`

The test environment hostname.

## Example usage
```yaml
uses: Scalr/gh-action-revizor@master
with:
    command: create
```