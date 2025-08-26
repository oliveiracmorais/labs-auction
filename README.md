# 🏆 Labs Auction - Sistema de Leilões Automatizados

Um sistema simples de leilões com fechamento automático baseado em tempo, desenvolvido em Go com MongoDB.

## 🎯 Objetivos do Projeto

Este projeto tem como objetivo demonstrar:

1. **⏰ Fechamento automático de leilões** com base em tempo configurável via variáveis de ambiente.
2. **🔄 Monitor de leilões expirados** usando uma *goroutine* que verifica periodicamente e fecha leilões vencidos.
3. **✅ Teste automatizado** validando que o fechamento ocorre de forma automática e confiável.

---

## 🛠️ Tecnologias Utilizadas

- **Go** (Golang 1.24+)
- **MongoDB** (com autenticação)
- **Docker + Docker Compose**
- **Gin** (API HTTP)
- **Testify** (testes)
- **Zap** (logs)

---

## 🚀 Como Executar em Ambiente de Desenvolvimento

### 1. Clone o repositório

```bash
git clone https://github.com/oliveiracmorais/labs-auction.git
cd labs-auction
```

### 2. Crie o ambiente de execução

```bash
docker-compose up -d monbodb
```

### 3. Execute o teste

```bash
go test -v ./internal/infra/database/auction -run TestCreateAuction_ClosesAutomatically
```
