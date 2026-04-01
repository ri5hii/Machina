#!/usr/bin/env python3

from __future__ import annotations

import csv
import random
import shutil
import string
from pathlib import Path


ROOT = Path(__file__).resolve().parent
DATA_DIR = ROOT / "tests" / "data"

SMALL_CSVS = {
    "sample.csv": {
        "header": ["id", "name", "department", "salary"],
        "rows": [
            ["1", "Alice", "Engineering", "95000"],
            ["2", "Bob", "Marketing", "72000"],
            ["3", "Charlie", "Finance", "81000"],
            ["4", "Diana", "Engineering", "99000"],
            ["5", "Eric", "Operations", "67000"],
        ],
    },
    "employees.csv": {
        "header": ["emp_id", "first_name", "last_name", "department", "role", "salary"],
        "rows": [
            ["E001", "  Alice  ", "Johnson", "  Engineering  ", "Senior Engineer", "95000"],
            ["E002", "BOB", "SMITH", "Marketing", "Manager", "72000"],
            ["E003", "  Carol  ", "White", "  Engineering  ", "Junior Engineer", "65000"],
            ["E004", "DAVE", "BROWN", "Sales", "Executive", "58000"],
            ["E005", "  Eve  ", "Green", "  HR  ", "Specialist", "61000"],
            ["E006", "FRANK", "BLACK", "Engineering", "Lead", "110000"],
            ["E007", "  Grace  ", "Kim", "  Finance  ", "Analyst", "68000"],
            ["E008", "HELEN", "LEE", "Operations", "Coordinator", "59000"],
            ["E009", "  Ian  ", "Young", "  Support  ", "Technician", "54000"],
            ["E010", "JACK", "HALL", "Product", "Director", "125000"],
        ],
    },
    "orders.csv": {
        "header": ["order_id", "customer", "product", "quantity", "status", "date"],
        "rows": [
            ["1001", "  alice johnson  ", "Widget Pro", "2", "  PENDING  ", "2024-01-15"],
            ["1002", "BOB SMITH", "Gadget Plus", "1", "SHIPPED", "2024-01-16"],
            ["1003", "  carol white  ", "Super Pen", "10", "  DELIVERED  ", "2024-01-17"],
            ["1004", "DAVE BROWN", "Notebook A5", "3", "PENDING", "2024-01-18"],
            ["1005", "  eve green  ", "Coffee Mug", "1", "  CANCELLED  ", "2024-01-19"],
            ["1006", "FRANK BLACK", "Laptop Stand", "1", "SHIPPED", "2024-01-20"],
            ["1007", "  grace kim  ", "USB Hub", "2", "  DELIVERED  ", "2024-01-21"],
            ["1008", "HELEN LEE", "Desk Lamp", "1", "SHIPPED", "2024-01-22"],
            ["1009", "  ian young  ", "Whiteboard", "2", "  PENDING  ", "2024-01-23"],
            ["1010", "JACK HALL", "Monitor Arm", "1", "DELIVERED", "2024-01-24"],
        ],
    },
    "products.csv": {
        "header": ["id", "name", "category", "price", "stock"],
        "rows": [
            ["1", "  Widget Pro  ", "Electronics", "  29.99  ", "150"],
            ["2", "GADGET PLUS", "Electronics", "49.99", "80"],
            ["3", "  Super Pen  ", "Stationery", "  2.49  ", "500"],
            ["4", "NOTEBOOK A5", "Stationery", "8.99", "300"],
            ["5", "  Coffee Mug  ", "Kitchen", "  12.99  ", "200"],
            ["6", "LAPTOP STAND", "Electronics", "39.99", "60"],
            ["7", "  USB Hub  ", "Electronics", "  19.99  ", "120"],
            ["8", "DESK LAMP", "Furniture", "24.99", "90"],
            ["9", "  Whiteboard  ", "Office", "  79.99  ", "35"],
            ["10", "MONITOR ARM", "Furniture", "59.99", "45"],
        ],
    },
    "users.csv": {
        "header": ["name", "email", "city", "country", "age"],
        "rows": [
            ["  Alice Johnson  ", "alice@example.com", "  New York  ", "USA", "30"],
            ["BOB SMITH", "bob@example.com", "London", "UK", "25"],
            ["  carol white  ", "carol@example.com", "  Paris  ", "France", "35"],
            ["DAVE BROWN", "dave@example.com", "Berlin", "Germany", "28"],
            ["  eve green  ", "eve@example.com", "  Tokyo  ", "Japan", "32"],
            ["FRANK BLACK", "frank@example.com", "Sydney", "Australia", "45"],
            ["  grace kim  ", "grace@example.com", "  Toronto  ", "Canada", "27"],
            ["HELEN LEE", "helen@example.com", "Seoul", "South Korea", "31"],
            ["  ian young  ", "ian@example.com", "  Singapore  ", "Singapore", "29"],
            ["JACK HALL", "jack@example.com", "Dublin", "Ireland", "41"],
        ],
    },
}

BIG_CSV_SPECS = [
    {
        "name": "employees_01.csv",
        "seed": 101,
        "rows": 10000,
        "header": ["emp_id", "first_name", "last_name", "department", "role", "salary", "status", "region"],
    },
    {
        "name": "orders_02.csv",
        "seed": 202,
        "rows": 10000,
        "header": ["order_id", "customer_id", "product", "quantity", "unit_price", "status", "priority", "region"],
    },
    {
        "name": "products_03.csv",
        "seed": 303,
        "rows": 10000,
        "header": ["product_id", "name", "category", "price", "stock", "status", "region", "priority"],
    },
    {
        "name": "users_04.csv",
        "seed": 404,
        "rows": 10000,
        "header": ["user_id", "username", "first_name", "last_name", "department", "role", "status", "region"],
    },
    {
        "name": "transactions_05.csv",
        "seed": 505,
        "rows": 10000,
        "header": ["txn_id", "account_id", "type", "amount", "status", "category", "priority", "region"],
    },
]

FIRST_NAMES = [
    "Alice", "Bob", "Carol", "Dave", "Eve", "Frank", "Grace", "Henry", "Iris", "Jack",
    "Karen", "Leo", "Mia", "Nina", "Omar", "Paul", "Quinn", "Rose", "Sam", "Tina",
    "Uma", "Victor", "Wendy", "Xena", "Yara", "Zoe",
]
LAST_NAMES = [
    "Johnson", "Smith", "White", "Brown", "Green", "Black", "Kim", "Lee", "Young", "Hall",
    "King", "Allen", "Martin", "Thomas", "Scott", "Adams", "Baker", "Davis", "Miller", "Jackson",
]
DEPARTMENTS = ["Engineering", "Marketing", "Sales", "Finance", "HR", "Operations", "Product", "Support", "Legal"]
ROLES = ["Engineer", "Senior Engineer", "Manager", "Coordinator", "Analyst", "Lead", "Director", "Specialist", "VP"]
STATUSES = ["active", "inactive", "pending", "suspended", "archived"]
PRIORITIES = ["low", "medium", "high", "critical"]
REGIONS = ["North", "South", "East", "West", "Central", "Remote"]
CATEGORIES = ["Software", "Hardware", "Services", "Consulting", "Training", "Support"]
TRANSACTION_TYPES = ["credit", "debit", "refund"]

OUTPUT_DIRS = [
    "csv/output",
    "encrypt/output",
    "encrypt/output/engine_1",
    "encrypt/output/engine_2",
    "encrypt/output/engine_3",
    "encrypt/output/engine_4",
    "encrypt/output/engine_5",
    "encrypt/output/recovery_1",
    "encrypt/output/recovery_2",
    "encrypt/output/recovery_3",
    "encrypt/output/recovery_4",
    "encrypt/output/recovery_final",
    "encrypt/output/start_flags_probe",
]

KEY_BYTES = bytes.fromhex("8eb037b9b42dac820aa0dc824f2d70120a916b2a6d459f75056949f5f9efab7e")


def reset_data_dir() -> None:
    """Remove any existing tests/data tree before regenerating it."""
    if DATA_DIR.exists():
        shutil.rmtree(DATA_DIR)
    DATA_DIR.mkdir(parents=True, exist_ok=True)


def write_csv(path: Path, header: list[str], rows: list[list[str]]) -> None:
    """Write one CSV file with a stable newline convention."""
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(header)
        writer.writerows(rows)


def write_small_csvs() -> None:
    """Write the small hand-authored CSV fixtures used by smoke flows."""
    base = DATA_DIR / "csv" / "input"
    for name, spec in SMALL_CSVS.items():
        write_csv(base / name, spec["header"], spec["rows"])


def random_wrapped(value: str) -> str:
    """Wrap generated values with spaces to exercise trim-style transforms."""
    return f"  {value}  "


def build_big_csv_rows(spec: dict[str, object]) -> list[list[str]]:
    """Build deterministic large CSV fixtures from a seeded pseudo-random stream."""
    rng = random.Random(int(spec["seed"]))
    rows: list[list[str]] = []
    name = str(spec["name"])
    count = int(spec["rows"])

    for index in range(1, count + 1):
        first = rng.choice(FIRST_NAMES)
        last = rng.choice(LAST_NAMES)
        department = rng.choice(DEPARTMENTS)
        role = rng.choice(ROLES)
        status = rng.choice(STATUSES)
        priority = rng.choice(PRIORITIES)
        region = rng.choice(REGIONS)
        category = rng.choice(CATEGORIES)

        if name == "employees_01.csv":
            rows.append([
                f"E{index:06d}",
                random_wrapped(first),
                last,
                random_wrapped(department),
                role,
                str(rng.randint(45000, 180000)),
                status,
                region,
            ])
        elif name == "orders_02.csv":
            package = f"{category} Package {rng.randint(0, 99)}"
            rows.append([
                f"ORD{index:07d}",
                f"CUST{rng.randint(0, 99999):05d}",
                random_wrapped(package),
                str(rng.randint(1, 500)),
                f"{rng.uniform(25, 750):.2f}",
                status,
                priority,
                region,
            ])
        elif name == "products_03.csv":
            product = f"{category} {role} Edition"
            rows.append([
                f"PRD{index:06d}",
                random_wrapped(product),
                category,
                f"{rng.uniform(100, 5000):.2f}",
                str(rng.randint(50, 10000)),
                status,
                region,
                priority,
            ])
        elif name == "users_04.csv":
            username = f"{first.lower()}.{last.lower()}{rng.randint(1, 999)}"
            rows.append([
                f"USR{index:06d}",
                random_wrapped(username),
                random_wrapped(first),
                last,
                department,
                role,
                status,
                region,
            ])
        elif name == "transactions_05.csv":
            rows.append([
                f"TXN{index:08d}",
                f"ACC{rng.randint(0, 999999):06d}",
                random_wrapped(rng.choice(TRANSACTION_TYPES)),
                f"{rng.uniform(1000, 99999):.2f}",
                status,
                category,
                priority,
                region,
            ])

    return rows


def write_big_csvs() -> None:
    """Write the large generated CSV fixtures used for higher-volume runs."""
    base = DATA_DIR / "csv" / "input"
    for spec in BIG_CSV_SPECS:
        write_csv(base / str(spec["name"]), list(spec["header"]), build_big_csv_rows(spec))


def write_root_input_csv() -> None:
    """Write the small root-level CSV fixture used by some older flows."""
    path = DATA_DIR / "input.csv"
    path.write_text("name,comment\nAlice,  hello world\nBob,  foo,bar\n", encoding="utf-8")


def write_key_file() -> None:
    """Write the deterministic AES key used by file encryption tests."""
    key_dir = DATA_DIR / "keys"
    key_dir.mkdir(parents=True, exist_ok=True)
    (key_dir / "default.key").write_bytes(KEY_BYTES)


def write_small_documents() -> None:
    """Write the ten small smoke-test text documents."""
    base = DATA_DIR / "encrypt" / "input"
    base.mkdir(parents=True, exist_ok=True)
    template = (
        "Document {n:02d}\n"
        "===========\n"
        "This is a sample text file for encryption testing.\n"
        "It contains multiple lines of data that will be encrypted.\n"
        "Line 4: some numbers 1234567890\n"
        "Line 5: some symbols !@#$%^&*()\n"
        "Line 6: project Machina file_encrypt smoke test\n"
    )
    for index in range(1, 11):
        (base / f"document_{index:02d}.txt").write_text(template.format(n=index), encoding="utf-8")


def build_large_document(index: int) -> str:
    """Build one deterministic 1 MiB-ish text document for encryption load tests."""
    rng = random.Random(8000 + index)
    alphabet = string.ascii_letters + string.digits + " \t"
    target_size = 1_048_626
    chunks: list[str] = []
    size = 0
    while size < target_size:
        line = "".join(rng.choice(alphabet) for _ in range(80)) + "\n"
        chunks.append(line)
        size += len(line)
    content = "".join(chunks)
    return content[:target_size]


def write_large_documents() -> None:
    """Write the hundred large deterministic text documents used for queue and batch tests."""
    base = DATA_DIR / "encrypt" / "input"
    base.mkdir(parents=True, exist_ok=True)
    for index in range(1, 101):
        (base / f"document_{index:03d}.txt").write_text(build_large_document(index), encoding="utf-8")


def create_output_dirs() -> None:
    """Create the output directory tree expected by the tests and CLI flows."""
    for relative in OUTPUT_DIRS:
        (DATA_DIR / relative).mkdir(parents=True, exist_ok=True)


def main() -> None:
    """Regenerate the repository test data tree from deterministic fixtures."""
    reset_data_dir()
    write_small_csvs()
    write_big_csvs()
    write_root_input_csv()
    write_key_file()
    write_small_documents()
    write_large_documents()
    create_output_dirs()

    print(f"Generated deterministic test data under {DATA_DIR}")
    print("- CSV fixtures written to tests/data/csv/input")
    print("- Encryption inputs written to tests/data/encrypt/input")
    print("- Output directory tree recreated under tests/data/encrypt/output")
    print("- Key written to tests/data/keys/default.key")


if __name__ == "__main__":
    main()
