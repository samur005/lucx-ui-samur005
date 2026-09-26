# Updates + vk-hash

Полная инструкция форка (канонический плейбук из двух частей): [FORK.md](FORK.md)  
Детали vk-hash: [VKHASH.md](VKHASH.md)

- Update в панели / `x-ui update` / стоковый `install.sh` качают релиз AlexeyLCP и **затирают** свой бинарник.
- GitHub Sync fork + **Discard** **стирает** коммиты форка (`vkhash.go`, Templates, …). Sync → **Update** допустим.
- Нужны апдейты AlexeyLCP: см. **Часть 1** в [FORK.md](FORK.md) (`git fetch upstream --tags` → merge `upstream/main` → push, сохранив EnsureVkHashes / vkcreator / Templates / `docs/FORK.md`).
- Затем **Часть 2** в [FORK.md](FORK.md): сборка frontend + Go на VPS и замена бинарника (порты не меняются).
- Вернуть только hash-хук: `curl -fsSL https://raw.githubusercontent.com/samur005/lucx-ui-samur005/feat/native-vk-hash-generator/scripts/apply-vkhash.sh | bash` (после merge PR — ветка `main`), затем снова Часть 2.
