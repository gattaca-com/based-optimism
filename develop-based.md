### in progress

- make sure you have set the upstream remote

```
git remote add upstream https://github.com/ethereum-optimism/optimism/
```

- only rebase from released tags of upstream
- check if you need rebasing with

```
git merge-base --is-ancestor v1.13.2  based/develop  && echo "Already rebased"
```

- if you need to rebase on a new tag

```
git checkout based/develop
git rebase --onto upstream/v1.13.3  upstream/v1.13.2  based/develop
git push --force-with-lease
```

- when ready push to based/main and make a new tag with v{op-node-version}-based-{based-version}. Make sure to bump {based-version} on based-op and op-geth whenever there's a change

```
git switch based/main
git merge --ff-only based/develop
git push
git tag v1.13.2-based-0.2.3
git push origin v1.13.2-based-0.2.3
```
