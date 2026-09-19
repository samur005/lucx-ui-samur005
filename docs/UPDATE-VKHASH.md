# Updates + vk-hash

Полная инструкция: [VKHASH.md](VKHASH.md)

- `x-ui update` и кнопка Update в панели качают релиз AlexeyLCP и **затирают** свой бинарник.
- GitHub Sync fork + Discard **стирает** `vkhash.go`.
- Нужны апдейты AlexeyLCP: `git fetch upstream && git merge upstream/main`, оставь `vkhash.go` и `EnsureVkHashes`.
- Вернуть патч: `curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/main/scripts/apply-vkhash.sh | bash`

Полная инструкция форка: [FORK.md](FORK.md).
