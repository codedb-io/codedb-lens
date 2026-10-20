# codeDB Lens

**Enxergue o que o seu banco não está mostrando.** Diagnóstico gratuito e somente leitura para SQL Server e PostgreSQL.

O Lens executa um catálogo de regras baseadas em evidências e gera um relatório HTML autônomo + JSON com
nota de A a F e selo de cobertura.

## Como ler o relatório

- **Nota (A–F):** ponderada pela severidade (Crítica 10, Alta 5, Média 2, Baixa 1) sobre as regras executadas.
  A ≥ 90, B ≥ 75, C ≥ 60, D ≥ 40, F < 40.
- **Cobertura:** regras executadas ÷ regras aplicáveis. Abaixo de 80%, a nota exibe o selo *análise parcial*,
  com o que faltou e a permissão que resolveria.
- **Confiança:** *Confirmado* (a evidência comprova), *Indício* (provável, precisa de análise humana) ou
  *Sem dados* (a regra não pôde rodar, e o motivo aparece).

O Lens mostra um retrato do momento. Para histórico, baseline e alertas, conheça o
[codeDB](https://codedb.com.br/?utm_source=lens&utm_medium=docs&utm_campaign=home).
