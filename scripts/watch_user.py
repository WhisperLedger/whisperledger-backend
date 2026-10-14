#!/usr/bin/env python3
"""
WhisperLedger Real-Time User & Expense Watcher
Run this anytime to check whether a newly registered or logged-in user
has their profile, household, and expenses properly populated.

Usage:
  ./scripts/watch_user.py                     # Shows latest 5 updated users
  ./scripts/watch_user.py --email <user_email> # Shows specific user details and expenses
"""

import sys
import argparse
import subprocess
import json
import urllib.request
from datetime import datetime

PROJECT_ID = "whisperledger-94715"
DATABASE_URL = f"https://firestore.googleapis.com/v1/projects/{PROJECT_ID}/databases/(default)/documents"

def get_access_token():
    try:
        res = subprocess.run(["/Users/tanmayagarwal/google-cloud-sdk/bin/gcloud", "auth", "print-access-token"], capture_output=True, text=True, check=True)
        return res.stdout.strip()
    except Exception as e:
        print(f"❌ Error getting gcloud token: {e}")
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
    parser = argparse.ArgumentParser(description="WhisperLedger User Watcher")
    parser.add_argument("--email", type=str, help="Search for a specific user email")
    args = parser.parse_args()

    token = get_access_token()
    users_resp = fetch_firestore("users", token)
    docs = [parse_doc(d) for d in users_resp.get("documents", [])]

    if args.email:
        target = args.email.strip().lower()
        matched = [d for d in docs if target in str(d.get("email", "")).lower()]
        if not matched:
            print(f"⚠️ No user found matching email '{args.email}'.")
            return
        
        user = matched[0]
        uid = user["_id"]
        print(f"\n========================================================")
        print(f"👤 USER PROFILE: {user.get('displayName') or user.get('name')} ({user.get('email')})")
        print(f"========================================================")
        print(f"• Firebase UID:      {uid}")
        print(f"• Created At:        {user.get('_createTime')}")
        print(f"• Last Updated:      {user.get('_updateTime')}")
        print(f"• Plan / Tier:       {user.get('plan', 'starter_free')}")
        print(f"• Status:            {user.get('status', 'active')}")
        print(f"• Push Token Ready:  {'Yes' if user.get('fcmToken') or user.get('expoPushToken') else 'No'}")
        
        # Check subcollection expenses
        exp_resp = fetch_firestore(f"users/{uid}/expenses", token)
        exp_docs = [parse_doc(e) for e in exp_resp.get("documents", [])]
        print(f"\n📊 EXPENSES LOGGED: {len(exp_docs)} total")
        print("--------------------------------------------------------")
        if not exp_docs:
            print("  (No transactions logged yet)")
        else:
            for exp in exp_docs[:5]:
                merchant = exp.get("merchant", "Unknown")
                amount = exp.get("amount", 0)
                cat = exp.get("category", "other")
                date = exp.get("date") or exp.get("_createTime")
                print(f"  • ₹{amount} at {merchant} [{cat}] on {date}")
            if len(exp_docs) > 5:
                print(f"  ... and {len(exp_docs) - 5} more expenses.")
    else:
        print("========================================================")
        print(f"👥 ALL REGISTERED USERS ({len(docs)} total):")
        print("========================================================")
        # Sort by update time
        docs.sort(key=lambda x: str(x.get("_updateTime", "")), reverse=True)
        for u in docs:
            uid = u["_id"]
            name = u.get("displayName") or u.get("name") or "User"
            email = u.get("email", "No Email")
            upd = u.get("_updateTime", "")[:19]
            print(f"• {name} | {email}")
            print(f"  UID: {uid} | Last Active: {upd}")
            print()

if __name__ == "__main__":
    main()
