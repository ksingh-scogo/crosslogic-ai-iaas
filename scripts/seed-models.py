#!/usr/bin/env python3
"""
Seed additional models to the database for testing the Launch UI
"""

import os
import psycopg2
from psycopg2.extras import execute_values
import sys

# Database connection from environment
DB_HOST = os.getenv("POSTGRES_HOST", "localhost")
DB_PORT = os.getenv("POSTGRES_PORT", "5432")
DB_NAME = os.getenv("POSTGRES_DB", "crosslogic")
DB_USER = os.getenv("POSTGRES_USER", "crosslogic")
DB_PASSWORD = os.getenv("POSTGRES_PASSWORD", "crosslogic123")

# Additional models to seed
ADDITIONAL_MODELS = [
    {
        "name": "meta-llama/Llama-3-8b-chat-hf",
        "family": "Llama",
        "size": "8B",
        "type": "chat",
        "context_length": 8192,
        "vram_required_gb": 16,
        "price_input_per_million": 0.05,
        "price_output_per_million": 0.05,
        "tokens_per_second_capacity": 100,
        "status": "active"
    },
    {
        "name": "meta-llama/Llama-3-70b-chat-hf",
        "family": "Llama",
        "size": "70B",
        "type": "chat",
        "context_length": 8192,
        "vram_required_gb": 80,
        "price_input_per_million": 0.60,
        "price_output_per_million": 0.60,
        "tokens_per_second_capacity": 50,
        "status": "active"
    },
    {
        "name": "mistralai/Mistral-7B-Instruct-v0.3",
        "family": "Mistral",
        "size": "7B",
        "type": "chat",
        "context_length": 32768,
        "vram_required_gb": 16,
        "price_input_per_million": 0.04,
        "price_output_per_million": 0.04,
        "tokens_per_second_capacity": 100,
        "status": "active"
    },
    {
        "name": "mistralai/Mixtral-8x7B-Instruct-v0.1",
        "family": "Mistral",
        "size": "8x7B",
        "type": "chat",
        "context_length": 32768,
        "vram_required_gb": 48,
        "price_input_per_million": 0.24,
        "price_output_per_million": 0.24,
        "tokens_per_second_capacity": 80,
        "status": "active"
    },
    {
        "name": "Qwen/Qwen2.5-7B-Instruct",
        "family": "Qwen",
        "size": "7B",
        "type": "chat",
        "context_length": 32768,
        "vram_required_gb": 16,
        "price_input_per_million": 0.04,
        "price_output_per_million": 0.04,
        "tokens_per_second_capacity": 100,
        "status": "active"
    },
    {
        "name": "Qwen/Qwen2.5-72B-Instruct",
        "family": "Qwen",
        "size": "72B",
        "type": "chat",
        "context_length": 32768,
        "vram_required_gb": 80,
        "price_input_per_million": 0.60,
        "price_output_per_million": 0.60,
        "tokens_per_second_capacity": 50,
        "status": "active"
    },
    {
        "name": "google/gemma-2-9b-it",
        "family": "Gemma",
        "size": "9B",
        "type": "chat",
        "context_length": 8192,
        "vram_required_gb": 20,
        "price_input_per_million": 0.06,
        "price_output_per_million": 0.06,
        "tokens_per_second_capacity": 90,
        "status": "active"
    },
    {
        "name": "google/gemma-2-27b-it",
        "family": "Gemma",
        "size": "27B",
        "type": "chat",
        "context_length": 8192,
        "vram_required_gb": 60,
        "price_input_per_million": 0.40,
        "price_output_per_million": 0.40,
        "tokens_per_second_capacity": 60,
        "status": "active"
    },
    {
        "name": "deepseek-ai/DeepSeek-Coder-V2-Instruct",
        "family": "DeepSeek",
        "size": "16B",
        "type": "chat",
        "context_length": 16384,
        "vram_required_gb": 32,
        "price_input_per_million": 0.14,
        "price_output_per_million": 0.14,
        "tokens_per_second_capacity": 70,
        "status": "active"
    }
]


def connect_db():
    """Connect to PostgreSQL database"""
    try:
        conn = psycopg2.connect(
            host=DB_HOST,
            port=DB_PORT,
            dbname=DB_NAME,
            user=DB_USER,
            password=DB_PASSWORD
        )
        return conn
    except Exception as e:
        print(f"❌ Failed to connect to database: {e}")
        sys.exit(1)


def seed_models(conn):
    """Seed models into the database"""
    cur = conn.cursor()
    
    # Check existing models
    cur.execute("SELECT name FROM models")
    existing_models = {row[0] for row in cur.fetchall()}
    print(f"📊 Found {len(existing_models)} existing models in database")
    
    # Filter out models that already exist
    new_models = [m for m in ADDITIONAL_MODELS if m["name"] not in existing_models]
    
    if not new_models:
        print("✅ All models already exist in database")
        return
    
    print(f"📝 Adding {len(new_models)} new models...")
    
    # Insert new models
    insert_query = """
        INSERT INTO models (
            name, family, size, type, context_length, vram_required_gb,
            price_input_per_million, price_output_per_million,
            tokens_per_second_capacity, status
        ) VALUES %s
        ON CONFLICT (name) DO NOTHING
    """
    
    values = [
        (
            m["name"], m["family"], m["size"], m["type"],
            m["context_length"], m["vram_required_gb"],
            m["price_input_per_million"], m["price_output_per_million"],
            m["tokens_per_second_capacity"], m["status"]
        )
        for m in new_models
    ]
    
    execute_values(cur, insert_query, values)
    conn.commit()
    
    print(f"✅ Successfully added {len(new_models)} models:")
    for m in new_models:
        print(f"   • {m['name']} ({m['size']}) - {m['vram_required_gb']}GB VRAM")
    
    cur.close()


def list_all_models(conn):
    """List all models in the database"""
    cur = conn.cursor()
    cur.execute("""
        SELECT name, family, size, vram_required_gb, status 
        FROM models 
        ORDER BY family, size
    """)
    
    print("\n" + "="*80)
    print("📋 All Models in Database:")
    print("="*80)
    
    current_family = None
    for row in cur.fetchall():
        name, family, size, vram, status = row
        if family != current_family:
            print(f"\n{family}:")
            current_family = family
        print(f"  • {name:<50} {size:<8} {vram}GB  [{status}]")
    
    print("="*80)
    cur.close()


def main():
    """Main function"""
    print("╔════════════════════════════════════════════════════════════════╗")
    print("║        CrossLogic - Model Database Seeder                      ║")
    print("╚════════════════════════════════════════════════════════════════╝")
    print()
    
    # Connect to database
    print(f"🔌 Connecting to PostgreSQL at {DB_HOST}:{DB_PORT}/{DB_NAME}...")
    conn = connect_db()
    print("✅ Connected successfully")
    print()
    
    try:
        # Seed models
        seed_models(conn)
        
        # List all models
        list_all_models(conn)
        
    except Exception as e:
        print(f"❌ Error: {e}")
        conn.rollback()
        sys.exit(1)
    finally:
        conn.close()
        print("\n✅ Done!")


if __name__ == "__main__":
    main()

