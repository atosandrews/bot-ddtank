# Olimpíadas Boom (Normal) headless

Terceiro bot (12–13/11/2026), no modelo do Pintinho: instância 6, Normal = 1; mapas 1165 (balões), 1167 (boxeador
"Juíz Ladrão Meka 6132") e 1168 ("Grupo de Fans (Direito)").

- `fazer_olimpiadas.py` gera `lua/instances/olimpiadas.lua` a partir do `pintinho/lua/instances/pintinho.lua` mais
  `config.lua.txt`, `baloes.lua.txt`, `turno_baloes.lua.txt` e `passo_chefe.lua.txt`.
- `relay-raw-hex.patch`: o relay da Olimpíadas usa uma cópia do `codigo` com esta mudança (subcomandos de luta
  desconhecidos levam `raw_hex`); é assim que o script lê os balões (91.48 objeto, 91.223 valor, 91.53 remove,
  91.104 placar).
- Estágio 1: as 4 contas atacam com +2 + tridente; o tiro passa pelos balões a até 40 px da trajetória (medido) e a
  mira escolhe o maior total líquido, recalculando a cada tiro extra.
- Estágio 2: só a líder, +1 + tridente + dano máximo; perto do chefe anda um pouco para longe.
- Estágio 3: vence ao matar a "Mesa dos Juízes" (1500, aparece por um turno); tiro alto pelas costas do chefe
  (lado oposto para onde ele está virado); com a Mesa viva não anda.
- Hospedagem: `../fly-olimpiadas` (app ddbot-olimpiadas). Teste local: `sonda.sh` no contêiner ddbot-olimpiadas-sonda.
- Contas e cookies (não versionar): `lider.yaml`, `convidadas1.yaml`, `convidadas2.yaml`, `sessao_*.json`
  (modelo em `lider.exemplo.yaml`).

13/11: o servidor de login s1.ddclassico.com passou a responder 403 (Cloudflare) ao login headless; os bots
estão parados até isso mudar.
