# Optimism Interop Monitor

## Purpose
The Optimism Interop Monitor is a service that monitors cross-chain interactions between OP Protocol chains to detect and report any invalid messages on chains. It helps ensure the reliability and correctness of cross-chain communication.

Interop Monitor's primary output is a collection of metrics which can alert operators for fast response and insight.
The design of this service follows the [Design Doc](https://github.com/ethereum-optimism/design-docs/pull/222),
and boilerplate structure was pulled from the adjacent `op-dripper`.

## Architecture
The service consists of several key components working together:

- A main service (`InteropMonitorService`) that coordinates everything
- Multiple `Finder` instances that scan chains for relevant transactions
- A `Watcher` that processes and tracks the state of cross-chain messages
- Metrics reporting for monitoring and alerting

The components communicate through channels and maintain state about message status.

## Watcher
The `Watcher` is responsible for tracking the status of cross-chain messages over time. It:

- Receives jobs from Finders by draining the Finders' outbox
- Processes each job to determine its current status (valid/invalid/missing)
- Maintains state about message status and updates
- One large channel as a looping queue of recurring jobs
- Emits metrics about message status
- Jobs are dropped once no longer needed for monitoring

### Updaters
Updaters are chain specific processors that take jobs and update them. They can be used to batch requests and run in parallel.
Currently the only updater is the Watcher itself, which is a bottleneck. Sub-processors should be implemented

## Finders
Finders scan individual chains for relevant cross-chain transactions. Each Finder:

- Subscribes to new blocks on its assigned chain
- Processes block receipts to identify cross-chain messages
- Creates jobs for each relevant transaction found
- Sends jobs to the Watcher for tracking
- Operates independently per chain

## Jobs
Jobs represent individual cross-chain messages that need to be tracked. A job contains:

- Timestamps for first/last seen
- Transaction hashes and Initiating/Executing identifiers
- Current status and status history

Jobs move through different states (unknown -> valid/invalid/missing) as the Watcher processes them.
