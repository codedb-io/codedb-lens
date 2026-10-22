# Como a nota é calculada

A nota, a cobertura e a confiança de cada achado são coisas diferentes. O relatório sempre mostra as três.

## Resultados de uma regra

Cada regra termina em um de quatro estados:

| Estado | Significado | Efeito na nota |
|---|---|---|
| **OK** | A regra rodou e não encontrou nada | Crédito total |
| **Achado** | A regra rodou e retornou linhas | Sem crédito (*Confirmado*) ou meio crédito (*Indício*) |
| **Sem dados** | Falta de permissão, timeout, erro ou recurso indisponível; o motivo aparece no relatório | Fica fora da nota e reduz a cobertura |
| **Não se aplica** | A regra não vale para esta versão ou plataforma (ex.: regra do SQL Agent no Azure SQL Database) | Fica fora das duas |

*Sem dados* nunca conta como OK. Uma regra que não rodou não melhora a nota.

## Nota

Toda regra tem uma severidade, e cada severidade tem um peso:

| Severidade | Peso |
|---|---|
| Crítica | 10 |
| Alta | 5 |
| Média | 2 |
| Baixa | 1 |

Sobre as regras que rodaram (OK ou Achado):

```
nota = 100 × Σ (peso × crédito) / Σ peso
crédito = 1 para OK, 0,5 para achado Indício, 0 para achado Confirmado
```

| Letra | Nota |
|---|---|
| A | 90 ou mais |
| B | 75 ou mais |
| C | 60 ou mais |
| D | 40 ou mais |
| F | abaixo de 40 |

Se nenhuma regra pôde rodar, a nota é **?**.

## Cobertura

```
cobertura = regras que rodaram / regras que se aplicam
```

Abaixo de 80%, o relatório mostra o selo **análise parcial** ao lado da nota: um A com 50% de cobertura só diz que
metade das verificações passou. Conceda as [permissões mínimas](permissions.md) para aumentá-la.

## Confiança

- **Confirmado:** os valores coletados comprovam o problema (ex.: nenhum backup full há 9 dias).
- **Indício:** os valores apontam um problema provável, que depende de contexto que o Lens não vê (ex.: configurações
  no padrão). Um indício custa metade do peso.

## Reprodutibilidade

Pesos, créditos e limites das letras ficam em [`rules/scoring.yaml`](https://github.com/codedb-io/codedb-lens/blob/main/rules/scoring.yaml),
não no código. Cada relatório registra a versão do Lens e uma impressão digital do **catálogo de regras**
(`catalog_version` no JSON): os mesmos dados com o mesmo catálogo sempre dão a mesma nota. O `compare` avisa quando
duas execuções usaram catálogos diferentes.
