#!/usr/bin/env bash

just forge-build
forge script -vvvvv scripts/deploy/InteropAlphanetMigration.s.sol:InteropAlphanetMigration $@
