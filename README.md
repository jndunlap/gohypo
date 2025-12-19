# GoHypo: General Purpose Causal Discovery & Falsification Engine

**GoHypo** is not a dashboard; it is an autonomous **Scientific Laboratory.** It is a high-performance engine built in Go that treats data analysis as a rigorous architectural process. By pairing a **Creative AI Scientist** (LLM) with a **Deterministic Skeptic** (Go Engine), GoHypo continuously mines datasets for non-obvious truths while programmatically hunting for reasons to reject them.

---

## 🔬 What is it?

GoHypo is a "math-first" engine designed to move beyond simple correlation. It operates on the principle of **Autonomous Causal Discovery.** Instead of a human analyst hunting for patterns, GoHypo ingests metadata and architects its own research laboratory to prove—or falsify—complex behavioral interactions.

### The "Greenfield" Philosophy

In GoHypo, the engine starts as a blank slate. It doesn't use generic, one-size-fits-all tests. Instead, it looks at your specific data and **blueprints the exact mathematical instruments** required to understand the "physics" of your variables.

---

## 🏗️ How it Works: The Discovery Loop

GoHypo operates in a continuous, 24/7 loop that bridges the gap between human-readable intuition and machine-executable rigor.

| **Stage**        | **Actor**             | **Action**                                       | **Output**                    |
| ---------------------- | --------------------------- | ------------------------------------------------------ | ----------------------------------- |
| **1. Ingest**    | **Go Engine**         | Resolves raw data into a canonical `MatrixBundle`.   | **Field Metadata**            |
| **2. Architect** | **AI Scientist**      | Analyzes metadata to find high-value "Hidden Truths."  | **Research Directive (JSON)** |
| **3. Build**     | **Go Engine**         | Dynamically assembles requested "Instruments."         | **Statistical Modules**       |
| **4. Referee**   | **Programmatic Gate** | Runs the "Ghost Test" to attempt falsification.        | **Pass/Fail Verdict**         |
| **5. Ledger**    | **Blueprint UI**      | Renders the validated finding as an engineering sheet. | **Validated Insight**         |

---

## 📋 The Research Directive (The Handshake)

At the heart of GoHypo is the  **Research Directive** . This is a JSON-based "Work Order" that bridges the gap between a business feeling and a technical build. It splits every discovery into a **Business Hypothesis** (the story) and a **Science Hypothesis** (the math).

### Example Directive

**JSON**

```
{
  "id": "HYP-001",
  "business_hypothesis": "Customers get 'Coupon Fatigue'—if we send too many discounts, they stop buying full-price items entirely.",
  "science_hypothesis": "A non-linear 'cliff' exists where full-price transaction volume drops exponentially after 4 coupon redemptions in a 30-day window.",
  "null_case": "Full-price purchasing frequency remains stable regardless of the number of discounts redeemed.",
  "validation_methods": [
    {
      "type": "Detector",
      "method_name": "Redemption_Decay_Monitor",
      "execution_plan": "Calculate the slope of full-price orders relative to coupon count. The test passes if the slope pivots sharply negative after the 4th redemption."
    },
    {
      "type": "Scanner",
      "method_name": "Ghost_Pattern_Shuffle",
      "execution_plan": "Randomize coupon counts across the user base 10,000 times. The pattern is validated only if the 'cliff' disappears in 99.9% of random shuffles."
    }
  ]
}
```

---

## 🛡️ The Falsification Engine (The Referee)

GoHypo’s primary mission is to **disprove itself.** Every hypothesis is accompanied by a **Null Case (The Ghost Test).** The Go-based **Referee** is a specialized gatekeeper that runs a battery of stress tests—Permutations, Bootstrapping, and Cross-Validation—specifically designed to prove that a "Discovery" is actually just random noise. If the pattern survives the Referee, it is stamped as **VALIDATED.**

---

## 🎨 The Blueprint Aesthetic

GoHypo abandons modern web "fluff" for a **Technical Vellum** UI designed for high-density information.

* **Instrument-Grade Detail:** Every panel is separated by 2px solid lines with "Drafting Overhangs" at the corners.
* **Blueprint Accents:** We use **Drafting Blue** for all active measurements, dimension lines, and "Work-in-Progress" research.
* **Engineering Title Blocks:** Every discovery is anchored by a formal title block containing the Project Name, Data Hash, and the Scientist's ID.

---

## 🚀 Getting Started

### Prerequisites

- Go 1.24+
- PostgreSQL (or Docker)
- OpenAI API Key

### Quick Start with Docker

1. **Start the Database:**
   ```bash
   make db-up
   # Or: docker-compose up -d postgres
   ```

2. **Set Environment Variables:**
   ```bash
   cp .env.example .env
   # Edit .env with your OpenAI API key and database URL
   ```

3. **Run the Application:**
   ```bash
   make run
   # Or: go run main.go
   ```

4. **Access the UI:**
   - Main Application: http://localhost:8081
   - Database Admin (optional): http://localhost:5050

### Manual Setup (Without Docker)

1. **Install PostgreSQL:**
   ```bash
   # macOS
   brew install postgresql
   brew services start postgresql

   # Ubuntu/Debian
   sudo apt install postgresql postgresql-contrib
   sudo systemctl start postgresql
   ```

2. **Create Database:**
   ```sql
   CREATE DATABASE gohypo;
   CREATE USER gohypo_user WITH PASSWORD 'gohypo_password';
   GRANT ALL PRIVILEGES ON DATABASE gohypo TO gohypo_user;
   ```

3. **Set Environment:**
   ```bash
   export DATABASE_URL="postgres://gohypo_user:gohypo_password@localhost:5432/gohypo?sslmode=disable"
   export OPENAI_API_KEY="your-api-key-here"
   ```

4. **Run:**
   ```bash
   go run main.go
   ```

### Development Commands

```bash
# Database management
make db-up          # Start database
make db-down        # Stop database
make db-reset       # Reset database (WARNING: destroys data)
make db-admin       # Start pgAdmin
make db-logs        # View database logs

# Application
make build          # Build binary
make run           # Run application
make test          # Run tests
make clean         # Clean build artifacts

# Full development setup
make dev           # Start everything
```

### Configuration

GoHypo uses the following environment variables:

- `DATABASE_URL`: PostgreSQL connection string
- `OPENAI_API_KEY`: Your OpenAI API key for research generation
- `LLM_MODEL`: OpenAI model (default: gpt-4-turbo-preview)
- `PROMPTS_DIR`: Directory containing research prompts
- `EXCEL_FILE`: Path to data file (CSV/Excel)
- `PORT`: Server port (default: 8081)

---

## 🔬 Research Workflow

1. **Connect Data:** Point GoHypo to any SQL, CSV, or Parquet source.
2. **Generate Metadata:** The engine automatically resolves your fields into the `MatrixBundle`.
3. **Initiate Research:** Click the **[INITIATE_SCAN]** button to activate the AI Scientist.
4. **Inspect Blueprints:** Review the generated Research Directives and authorize the Go Engine to build the necessary validation instruments.
