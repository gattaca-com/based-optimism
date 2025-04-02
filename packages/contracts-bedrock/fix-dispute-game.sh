#!/usr/bin/env bash

just forge-build

forge script -vv scripts/deploy/FixDisputeGame.s.sol:FixDisputeGame $@
