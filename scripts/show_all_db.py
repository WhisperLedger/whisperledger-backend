#!/usr/bin/env python3
"""
WhisperLedger Comprehensive Database Viewer
Fetches and displays ALL records across all collections & subcollections:
- Users (Profiles, Plans, Push tokens, Timestamps)
- Households (Invite codes, Members)
- Expenses (All transactions, Categories, Amounts, Merchants, Dates)
- Broadcast History (Push Notifications, Delivery stats)

Usage:
  ./scripts/show_all_db.py            # Formatted readable summary & ledger table
  ./scripts/show_all_db.py --json     # Dumps complete raw JSON of the entire DB
"""

import sys
import json
import argparse
import subprocess
import urllib.request
from datetime import datetime

PROJECT_ID = "whisperledger-94715"
DATABASE_URL = f"https://firestore.googleapis.com/v1/projects/{PROJECT_ID}/databases/(default)/documents"

def get_access_token():
    try:
        res = subprocess.run(["/Users/tanmayagarwal/google-cloud-sdk/bin/gcloud", "auth", "print-access-token"], capture_output=True, text=True, check=True)
        return res.stdout.strip()
    except Exception as e:
        print(f"❌ Error obtaining Google Cloud token: {e}")
        sys.exit(1)

def fetch_firestore(path, token):
    url = f"{DATABASE_URL}/{path}"
    req = urllib.request.Request(url, headers={"Authorization": f"Bearer {token}"})
    try:
        with urllib.request.urlopen(req) as response:
            return json.loads(response.read().decode("utf-8"))
    except Exception:
        return {}

def parse_val(val_obj):
    if not isinstance(val_obj, dict):
        return val_obj
    for k in ["stringValue", "booleanValue"]:
        if k in val_obj: return val_obj[k]
    if "integerValue" in val_obj: return int(val_obj["integerValue"])
    if "doubleValue" in val_obj: return float(val_obj["doubleValue"])
    if "timestampValue" in val_obj: return val_obj["timestampValue"]
    if "mapValue" in val_obj:
        return {k: parse_val(v) for k, v in val_obj["mapValue"].get("fields", {}).items()}
    if "arrayValue" in val_obj:
        return [parse_val(v) for v in val_obj["arrayValue"].get("values", [])]
    return val_obj

def parse_doc(doc_dict):
    data = {}
    for k, v in doc_dict.get("fields", {}).items():
        data[k] = parse_val(v)
    data["_id"] = doc_dict.get("name", "").split("/")[-1]
    data["_createTime"] = doc_dict.get("createTime")
    data["_updateTime"] = doc_dict.get("updateTime")
    return data

def main():
    parser = argparse.ArgumentParser(description="View all data in WhisperLedger database")
    parser.add_argument("--json", action="store_true", help="Output complete raw JSON")
    args = parser.parse_args()

    token = get_access_token()

    # 1. Fetch Users
    users_raw = fetch_firestore("users", token).get("documents", [])
    users = [parse_doc(d) for d in users_raw]

    # 2. Fetch Households
    hh_raw = fetch_firestore("households", token).get("documents", [])
    households = [parse_doc(d) for d in hh_raw]

    # 3. Fetch Broadcasts
    bc_raw = fetch_firestore("broadcast_history", token).get("documents", [])
    broadcasts = [parse_doc(d) for d in bc_raw]

    # 4. Fetch All Expenses across all users
    all_expenses = []
    for u in users:
        uid = u["_id"]
        exp_raw = fetch_firestore(f"users/{uid}/expenses", token).get("documents", [])
        for e in exp_raw:
            parsed = parse_doc(e)
            parsed["_userId"] = uid
            parsed["_userEmail"] = u.get("email", "Unknown")
            parsed["_userName"] = u.get("displayName") or u.get("name") or "User"
            all_expenses.append(parsed)

    if args.json:
        dump = {
            "metadata": {
                "project": PROJECT_ID,
                "timestamp": datetime.now().isoformat(),
                "counts": {
                    "users": len(users),
                    "households": len(households),
                    "expenses": len(all_expenses),
                    "broadcasts": len(broadcasts)
                }
            },
            "users": users,
            "households": households,
            "expenses": all_expenses,
            "broadcasts": broadcasts
        }
        print(json.dumps(dump, indent=2))
        return

    # Formatted terminal display
    total_volume = sum(float(e.get("amount", 0)) for e in all_expenses)

    print("==========================================================================================")
    print(f"📦 WHISPERLEDGER COMPLETE DATABASE VIEW (Project: {PROJECT_ID})")
    print(f"   Generated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print(f"   Summary: {len(users)} Users · {len(households)} Households · {len(all_expenses)} Expenses · ₹{total_volume:,.2f} Volume")
    print("==========================================================================================")

    print(f"\n👤 [1] USERS DIRECTORY ({len(users)} accounts)")
    print(f"{'UID':<30} | {'NAME':<20} | {'EMAIL':<30} | {'PLAN':<8} | {'STATUS':<8}")
    print("-" * 105)
    for u in users:
        uid = u["_id"]
        name = (u.get("displayName") or u.get("name") or "Unnamed")[:18]
        email = str(u.get("email", "No Email"))[:28]
        plan = (u.get("plan") or ("pro" if u.get("isPro") else "free"))[:6]
        status = u.get("status", "active")[:8]
        print(f"{uid:<30} | {name:<20} | {email:<30} | {plan:<8} | {status:<8}")

    print(f"\n🏠 [2] HOUSEHOLDS ({len(households)} groups)")
    print(f"{'ID':<15} | {'NAME':<25} | {'INVITE CODE':<15} | {'MEMBERS'}")
    print("-" * 75)
    for h in households:
        hid = h["_id"][:14]
        name = str(h.get("name", "Unnamed"))[:24]
        code = str(h.get("inviteCode") or h.get("code") or "N/A")[:14]
        members = h.get("members", [])
        m_count = len(members) if isinstance(members, list) else 1
        print(f"{hid:<15} | {name:<25} | {code:<15} | {m_count} member(s)")

    print(f"\n💳 [3] EXPENSES LEDGER ({len(all_expenses)} total transactions across all users)")
    print(f"{'DATE':<12} | {'AMOUNT':<10} | {'CATEGORY':<14} | {'MERCHANT / NOTE':<25} | {'USER'}")
    print("-" * 95)
    # Show newest first
    all_expenses.sort(key=lambda x: str(x.get("date") or x.get("_createTime") or ""), reverse=True)
    for e in all_expenses[:25]:
        d = str(e.get("date") or e.get("_createTime") or "")[:10]
        amt = f"₹{float(e.get('amount', 0)):,.2f}"
        cat = str(e.get("category", "other"))[:13]
        desc = str(e.get("merchant") or e.get("note") or "Unknown")[:24]
        uname = e.get("_userName", "")[:18]
        print(f"{d:<12} | {amt:<10} | {cat:<14} | {desc:<25} | {uname}")

    if len(all_expenses) > 25:
        print(f"... and {len(all_expenses) - 25} more transactions.")

    print("\n==========================================================================================")
    print("💡 Tip: Run `./scripts/show_all_db.py --json` to export the full JSON database dump.")
    print("==========================================================================================")

if __name__ == "__main__":
    main()
