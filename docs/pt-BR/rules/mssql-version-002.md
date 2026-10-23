# MSSQL-VERSION-002 · Build atrás da última CU

| Severidade | Confiança | Permissão | Aplica-se a |
|---|---|---|---|
| Média | Confirmado (CU com mais de 180 dias de atraso) · Indício (build mais novo que a tabela embutida) | `public` | SQL Server 2017–2025, RDS (não Azure SQL Database / MI, que não têm versão) |

## O que verifica

O build instalado (`SERVERPROPERTY('ProductVersion')`) contra o histórico de versões da Microsoft embutido no Lens
(RTM e todas as atualizações cumulativas do 2017, 2019, 2022 e 2025). Um build entre duas CUs, como um build só de
segurança (GDR), conta como a CU imediatamente anterior. O achado aparece quando a CU em uso foi lançada mais de 180 dias
antes da última CU da sua versão (`max_age_days`). Mostra o build instalado, a CU correspondente, a última CU e sua data,
e os dias de atraso.

## Por que importa

As atualizações cumulativas trazem as correções de bugs e de segurança da sua versão. Um build muitos meses atrasado
mantém problemas conhecidos em aberto e torna o próximo patch um salto maior.

## Próxima ação

Teste a última atualização cumulativa numa cópia fora de produção e agende a aplicação numa janela de manutenção.

## Como validar

Depois de aplicar o patch, rode `SELECT SERVERPROPERTY('ProductVersion'), SERVERPROPERTY('ProductUpdateLevel')` e confirme que o build bate com o CU ou GDR alvo. Confira no error log do SQL Server se não há erros na subida e se jobs e membros de replicação ou AG voltaram saudáveis. Rode o codeDB Lens de novo: a regra deve ficar OK.

## Limitações

- A tabela de builds vem desta versão do Lens; atualize o Lens para renová-la.
- Um build mais novo que a tabela é reportado como *Indício* (a tabela pode estar desatualizada), nunca como OK.
- Versões anteriores ao 2017 não são cobertas; [MSSQL-VERSION-001](mssql-version-001.md) reporta o fim do suporte delas.
