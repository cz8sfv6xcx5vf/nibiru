# Chaosnet

- [Chaosnet](#chaosnet)
  - [What is Chaosnet?](#what-is-chaosnet)
  - [How to run "chaosnet"](#how-to-run-chaosnet)
  - [How to force pull images from the registry](#how-to-force-pull-images-from-the-registry)
  - [IBC Commands](#ibc-commands)
  - [Endpoints](#endpoints)
  - [FAQ](#faq)
    - [`make chaosnet` says that "Additional property name is not allowed"](#make-chaosnet-says-that-additional-property-name-is-not-allowed)
    - [Does data persist between runs?](#does-data-persist-between-runs)
    - [My `make chaosnet` takes forever to run](#my-make-chaosnet-takes-forever-to-run)

## What is Chaosnet?

Chaosnet is an expanded version of localnet that runs:

- two validators (nibiru-0 and nibiru-1)
- pricefeeders for each validator
- a hermes relayer between the two validators
- a faucet
- a postgres:14 database
- a heartmonitor instance
- a liquidator instance
- a graphql server

## How to run "chaosnet"

1. Make sure you have [Docker](https://docs.docker.com/engine/install/) installed and running
2. Make sure you have `make` installed
3. Docker login to ghcr.io

```bash
docker login ghcr.io
```

Enter your GitHub username for the `username` field, and your personal access token for the password.

4. Run `make chaosnet`

Note that this will take a while the first time you run it, as it will need to pull all the images from the registry, build the chaonset image locally, and set up the IBC channel (which has a lot of round trip packet commits).

## How to force pull images from the registry

By default, most images (heart-monitor, liquidator, etc.) are cached locally and won't re-fetch from upstream registries. To force a pull, you can run

```sh
make chaosnet-build
```

## IBC Commands

To send an IBC transfer from nibiru-0 to nibiru-1, run:

1. SSH into nibiru-0

    ```sh
    docker compose -f ./contrib/docker-compose/docker-compose-chaosnet.yml exec -it nibiru-0 /bin/ash
    ```

2. Transfer tokens from nibiru-0 to nibiru-1

    ```sh
    nibid tx ibc-transfer transfer transfer \
    channel-0 \
    nibi18mxturdh0mjw032c3zslgkw63cukkl4q5skk8g \
    1000000unibi \
    --from validator \
    --fees 5000unibi \
    --yes | jq
    ```

3. In a new shell, SSH into nibiru-1

    ```sh
    docker compose -f ./contrib/docker-compose/docker-compose-chaosnet.yml exec -it nibiru-1 /bin/ash
    ```

4. Query the balance of nibiru-1

    ```sh
    # set the config since nibiru-1 has different ports
    nibid config node "http://localhost:36657"

    nibid q bank balances $(nibid keys show validator -a) | jq
    ```

    Output:

    ```json
    {
      "balances": [
        {
          "denom": "ibc/9BEE732637B12723D26E365D19CCB624587CE6254799EEE7C5F77B587BD677B0",
          "amount": "1000000"
        },
        {
          "denom": "unibi",
          "amount": "9999100000000"
        }
      ],
      "pagination": {
        "next_key": null,
        "total": "0"
      }
    }
    ```

5. Send tokens from nibiru-1 to nibiru-0

    ```sh
    nibid tx ibc-transfer transfer transfer \
    channel-0 \
    nibi1zaavvzxez0elundtn32qnk9lkm8kmcsz44g7xl \
    5555unibi \
    --from validator \
    --fees 5000unibi \
    --yes | jq
    ```

6. Go back to the nibiru-0 and query the balance

    ```sh
    nibid q bank balances $(nibid keys show validator -a) | jq
    ```

    Output:

    ```json
    {
      "balances": [
        {
          "denom": "ibc/9BEE732637B12723D26E365D19CCB624587CE6254799EEE7C5F77B587BD677B0",
          "amount": "5555"
        },
        {
          "denom": "unibi",
          "amount": "9999098995000"
        }
      ],
      "pagination": {
        "next_key": null,
        "total": "0"
      }
    }
    ```

7. Send IBC tokens back to nibiru-1

    ```sh
    nibid tx ibc-transfer transfer transfer \
    channel-0 \
    nibi18mxturdh0mjw032c3zslgkw63cukkl4q5skk8g \
    5555ibc/9BEE732637B12723D26E365D19CCB624587CE6254799EEE7C5F77B587BD677B0 \
    --from validator \
    --fees 5000unibi \
    --yes | jq
    ```

8. Verify tokens are sent

    ```sh
    nibid q bank balances $(nibid keys show validator -a) | jq
    ```

    Output:

    ```json
    {
      "balances": [
        {
          "denom": "unibi",
          "amount": "9999098990000"
        }
      ],
      "pagination": {
        "next_key": null,
        "total": "0"
      }
    }
    ```

9. Back in the nibiru-1 shell, send tokens back to nibiru-0

    ```sh
    nibid tx ibc-transfer transfer transfer \
    channel-0 \
    nibi1zaavvzxez0elundtn32qnk9lkm8kmcsz44g7xl \
    1000000ibc/9BEE732637B12723D26E365D19CCB624587CE6254799EEE7C5F77B587BD677B0 \
    --from validator \
    --fees 5000unibi \
    --yes | jq
    ```
  
10. Verify tokens are sent

    ```sh
    nibid q bank balances $(nibid keys show validator -a) | jq
    ```

    Output:

    ```json
    {
      "balances": [
        {
          "denom": "unibi",
          "amount": "9999099990000"
        }
      ],
      "pagination": {
        "next_key": null,
        "total": "0"
      }
    }
    ```

## Endpoints

- `http://localhost:5555` -> GraphQL server
- `http://localhost:8000` -> Faucet server (HTTP POST only)
-
- `http://localhost:26657` -> nibiru-0 Tendermint RPC server
- `tcp://localhost:9090` -> nibiru-0 Cosmos SDK gRPC server
- `http://localhost:1317` -> nibiru-0 Cosmos SDK LCD (REST) server
-
- `http://localhost:36657` -> nibiru-1 Tendermint RPC server
- `tcp://localhost:19090` -> nibiru-1 Cosmos SDK gRPC server
- `http://localhost:11317` -> nibiru-1 Cosmos SDK LCD (REST) server

## FAQ

### `make chaosnet` says that "Additional property name is not allowed"

Make sure to update your docker application to version >=23.0.1

### Does data persist between runs?

No, all volumes are deleted and recreated every time you run `make chaosnet`. This is to ensure that you always start with a clean network.

### My `make chaosnet` takes forever to run

It usually takes a few minutes to set everything up and create the IBC channels. If it takes more than 5 minutes, then check the logs of the chaosnet containers to see if any step failed. Reach out to <kevin@nibiru.org> for help.
