import { Select } from "antd";
import { useTranslation } from "react-i18next";

const languages = [
  { label: "English", value: "en" },
  { label: "简体中文", value: "zh-CN" },
  { label: "正體中文", value: "zh-TW" },
  { label: "日本語", value: "ja" },
];

export default function LanguageSelector() {
  const { i18n } = useTranslation();

  const handleChange = (value: string) => {
    void i18n.changeLanguage(value);
    localStorage.setItem("language", value);
  };

  return (
    <Select
      value={i18n.language}
      onChange={handleChange}
      options={languages}
      style={{ width: 120 }}
      variant="borderless"
    />
  );
}
