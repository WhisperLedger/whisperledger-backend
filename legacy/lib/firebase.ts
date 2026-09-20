import { initializeApp } from "firebase/app";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { getAuth, getReactNativePersistence, initializeAuth } from "firebase/auth";
import { getFirestore } from "firebase/firestore";

// TODO: Replace with your actual Firebase config from the Firebase Console
const firebaseConfig = {
  apiKey: "AIzaSyDyWPFvO_MDqJbRdMjIv0WUJP4wO93dFnA",
  authDomain: "whisperledger-94715.firebaseapp.com",
  projectId: "whisperledger-94715",
  storageBucket: "whisperledger-94715.appspot.com",
  messagingSenderId: "1064792050841",
  appId: "1:1064792050841:web:9e30d27d9f6aa2e4fe3c08"
};

const app = initializeApp(firebaseConfig);
export const auth = (() => {
  try {
    return initializeAuth(app, {
      persistence: getReactNativePersistence(AsyncStorage),
    });
  } catch {
    return getAuth(app);
  }
})();
export const db = getFirestore(app);
