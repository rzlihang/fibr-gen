import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import en from "../locales/en/translation.json";
import zhCN from "../locales/zh-CN/translation.json";
import zhTW from "../locales/zh-TW/translation.json";
import ja from "../locales/ja/translation.json";

const resources = {
  en: { translation: en },
  "zh-CN": { translation: zhCN },
  "zh-TW": { translation: zhTW },
  ja: { translation: ja },
};

const savedLanguage = localStorage.getItem("language") || "en";

void i18n.use(initReactI18next).init({
  resources,
  lng: savedLanguage,
  fallbackLng: "en",
  interpolation: {
    escapeValue: false,
  },
});

export default i18n;
