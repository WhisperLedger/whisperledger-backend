#!/usr/bin/env python3
"""
WhisperLedger Firebase -> PostgreSQL Data Migration Engine
Extracts all real Users, Expenses, Households, and Settlements from Firebase Firestore
(project: whisperledger-94715) and generates structured SQL seeds and JSON dumps.
"""

import sys
import os
import json
import uuid
import subprocess
import urllib.request
import urllib.error
from datetime import datetime

PROJECT_ID = "whisperledger-94715"
DATABASE_URL = f"https://firestore.googleapis.com/v1/projects/{PROJECT_ID}/databases/(default)/documents"

def get_access_token():
    try:
        res = subprocess.run(["/Users/tanmayagarwal/google-cloud-sdk/bin/gcloud", "auth", "print-access-token"], capture_output=True, text=True, check=True)
        return res.stdout.strip()
    except Exception as e:
        print(f"Error getting gcloud token: {e}")
        sys.exit(1)

def fetch_firestore(path, token):
    url = f"{DATABASE_URL}/{path}"
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    try:
        with urllib.request.urlopen(req) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return {}
        print(f"HTTP error fetching {path}: {e.code} {e.reason}")
        return {}
    except Exception as e:
        print(f"Error fetching {path}: {e}")
        return {}

def parse_val(val_obj):
    if not isinstance(val_obj, dict):
        return val_obj
    if "stringValue" in val_obj:
        return val_obj["stringValue"]
    if "integerValue" in val_obj:
        return int(val_obj["integerValue"])
    if "doubleValue" in val_obj:
        return float(val_obj["doubleValue"])
    if "booleanValue" in val_obj:
        return val_obj["booleanValue"]
    if "timestampValue" in val_obj:
        return val_obj["timestampValue"]
    if "mapValue" in val_obj:
        res = {}
        for k, v in val_obj["mapValue"].get("fields", {}).items():
            res[k] = parse_val(v)
        return res
    if "arrayValue" in val_obj:
        return [parse_val(v) for v in val_obj["arrayValue"].get("values", [])]
    return str(val_obj)

def main():
    print("==========================================================")
    print("🔄 Starting WhisperLedger Firebase to PostgreSQL Migration")
    print(f"📦 Source Firebase Project: {PROJECT_ID}")
    print("==========================================================")

    token = get_access_token()

    # 1. Fetch Users
    print("\n1️⃣  Fetching Users...")
    users_raw = fetch_firestore("users", token).get("documents", [])
    users = []
    user_id_map = {} # firebase_uid -> postgres_uuid

    for doc in users_raw:
        doc_id = doc["name"].split("/")[-1]
        fields = {k: parse_val(v) for k, v in doc.get("fields", {}).items()}
        pg_uuid = str(uuid.uuid5(uuid.NAMESPACE_DNS, doc_id))
        user_id_map[doc_id] = pg_uuid

        email = fields.get("email") or f"{doc_id}@whisperledger.app"
        name = fields.get("name") or "WhisperLedger Member"
        created_at = doc.get("createTime", datetime.utcnow().isoformat() + "Z")

        users.append({
            "firebase_uid": doc_id,
            "postgres_id": pg_uuid,
            "email": email,
            "name": name,
            "username": fields.get("username", ""),
            "avatar_url": fields.get("avatarUri", ""),
            "created_at": created_at,
            "role": "admin" if email == "agarwaltanmay401@gmail.com" else "user"
        })
        print(f"   ✓ User: {name} ({email}) -> UUID: {pg_uuid}")

    print(f"Total Users extracted: {len(users)}")

    # 2. Fetch Expenses for each user
    print("\n2️⃣  Fetching Real Expenses...")
    all_expenses = []
    for u in users:
        fb_uid = u["firebase_uid"]
        expenses_raw = fetch_firestore(f"users/{fb_uid}/expenses", token).get("documents", [])
        print(f"   Reading expenses for {u['name']} ({len(expenses_raw)} found)...")
        for doc in expenses_raw:
            exp_id = doc["name"].split("/")[-1]
            fields = {k: parse_val(v) for k, v in doc.get("fields", {}).items()}
            pg_exp_uuid = str(uuid.uuid5(uuid.NAMESPACE_DNS, exp_id))

            amt = float(fields.get("amount", 0))
            category = fields.get("categoryId") or fields.get("category") or "other"
            note = fields.get("note") or fields.get("merchant") or "Expense"
            payment_mode = fields.get("paymentMode") or "UPI"
            date_str = fields.get("date") or doc.get("createTime")

            # Outflow partitioning logic
            outflow_type = "true_personal"
            personal_share = amt
            recoverable = 0.0

            if fields.get("isShared") or fields.get("householdId"):
                outflow_type = "shared_household"
                personal_share = amt / 2.0
                recoverable = amt / 2.0
            elif fields.get("isRecoverable") or fields.get("frontedFor"):
                outflow_type = "recoverable"
                personal_share = 0.0
                recoverable = amt

            all_expenses.append({
                "firebase_id": exp_id,
                "postgres_id": pg_exp_uuid,
                "user_id": u["postgres_id"],
                "user_name": u["name"],
                "amount": amt,
                "personal_share": personal_share,
                "recoverable_amount": recoverable,
                "outflow_type": outflow_type,
                "category": category,
                "merchant": note,
                "payment_mode": payment_mode,
                "date": date_str,
                "note": note,
                "created_at": doc.get("createTime")
            })

    print(f"Total Real Expenses extracted: {len(all_expenses)}")

    # 3. Fetch Households
    print("\n3️⃣  Fetching Households & Group Ledgers...")
    households_raw = fetch_firestore("households", token).get("documents", [])
    households = []
    household_members = []

    for doc in households_raw:
        h_id = doc["name"].split("/")[-1]
        fields = {k: parse_val(v) for k, v in doc.get("fields", {}).items()}
        pg_h_uuid = str(uuid.uuid5(uuid.NAMESPACE_DNS, h_id))

        creator_fb_uid = fields.get("createdBy", "")
        creator_pg_id = user_id_map.get(creator_fb_uid) or users[0]["postgres_id"]
        h_name = fields.get("name") or f"Household {h_id}"
        invite_code = fields.get("inviteCode") or h_id

        households.append({
            "firebase_id": h_id,
            "postgres_id": pg_h_uuid,
            "name": h_name,
            "invite_code": invite_code,
            "created_by": creator_pg_id,
            "created_at": doc.get("createTime")
        })
        print(f"   ✓ Household: {h_name} [Code: {invite_code}]")

        # Members
        member_uids = fields.get("memberUids", [])
        if isinstance(member_uids, list):
            for m_uid in member_uids:
                if m_uid in user_id_map:
                    household_members.append({
                        "household_id": pg_h_uuid,
                        "user_id": user_id_map[m_uid],
                        "role": "admin" if m_uid == creator_fb_uid else "member"
                    })

    print(f"Total Households: {len(households)}, Members: {len(household_members)}")

    # 4. Save to JSON Dump
    dump_data = {
        "source": "Firebase Firestore whisperledger-94715",
        "migrated_at": datetime.utcnow().isoformat() + "Z",
        "users": users,
        "expenses": all_expenses,
        "households": households,
        "household_members": household_members
    }

    dump_path = "/Users/tanmayagarwal/TanmayProjects/whisperledger-backend/data/firebase_migrated_data.json"
    with open(dump_path, "w", encoding="utf-8") as f:
        json.dump(dump_data, f, indent=2)
    print(f"\n💾 Saved full JSON dump to: {dump_path}")

    # 5. Generate SQL Seed Script
    sql_path = "/Users/tanmayagarwal/TanmayProjects/whisperledger-backend/migrations/000002_firebase_seed.up.sql"
    with open(sql_path, "w", encoding="utf-8") as f:
        f.write("-- WhisperLedger Real Firebase Data Seed Migration\n")
        f.write(f"-- Generated on {datetime.utcnow().isoformat()}Z from {PROJECT_ID}\n\n")

        f.write("-- 1. Insert Real Users\n")
        for u in users:
            email = u["email"].replace("'", "''")
            name = u["name"].replace("'", "''")
            avatar = u["avatar_url"].replace("'", "''")
            f.write(f"INSERT INTO users (id, email, password_hash, full_name, avatar_url, role, is_active) "
                    f"VALUES ('{u['postgres_id']}', '{email}', '$2a$10$realfirebaseuserhashplaceholdersalt', '{name}', '{avatar}', '{u['role']}', true) "
                    f"ON CONFLICT (email) DO UPDATE SET full_name = EXCLUDED.full_name, avatar_url = EXCLUDED.avatar_url;\n")

        f.write("\n-- 2. Insert Real Households\n")
        for h in households:
            h_name = h["name"].replace("'", "''")
            code = h["invite_code"].replace("'", "''")
            f.write(f"INSERT INTO households (id, name, invite_code, currency, created_by) "
                    f"VALUES ('{h['postgres_id']}', '{h_name}', '{code}', 'INR', '{h['created_by']}') "
                    f"ON CONFLICT (invite_code) DO NOTHING;\n")

        f.write("\n-- 3. Insert Household Members\n")
        for m in household_members:
            f.write(f"INSERT INTO household_members (household_id, user_id, role) "
                    f"VALUES ('{m['household_id']}', '{m['user_id']}', '{m['role']}') "
                    f"ON CONFLICT (household_id, user_id) DO NOTHING;\n")

        f.write("\n-- 4. Insert Real Expenses\n")
        for e in all_expenses:
            merchant = e["merchant"].replace("'", "''")
            note = e["note"].replace("'", "''")
            f.write(f"INSERT INTO expenses (id, user_id, amount, personal_share, recoverable_amount, outflow_type, category, merchant, payment_method, note) "
                    f"VALUES ('{e['postgres_id']}', '{e['user_id']}', {e['amount']}, {e['personal_share']}, {e['recoverable_amount']}, '{e['outflow_type']}', '{e['category']}', '{merchant}', '{e['payment_mode']}', '{note}') "
                    f"ON CONFLICT (id) DO NOTHING;\n")

    print(f"📄 Generated SQL Migration to: {sql_path}")
    print("\n✅ Migration complete! Real Firebase data successfully converted.")

if __name__ == "__main__":
    main()
