import { ConfigProvider } from "antd";
import { BrowserRouter, Routes, Route } from "react-router";
import { useTranslation } from "react-i18next";
import HomePage from "./pages/HomePage";
import GeneratePage from "./pages/GeneratePage";
import ConfigBuilderPage from "./pages/ConfigBuilderPage";
import LanguageSelector from "./components/LanguageSelector";

function App() {
  const { t } = useTranslation();
  return (
    <ConfigProvider>
      <BrowserRouter>
        <div className="fixed top-4 right-4 z-50 flex items-center gap-2">
          <a
            href="https://github.com/rzlihang/fibr-gen/blob/main/docs/guide.md"
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm text-gray-500 hover:text-gray-800 transition-colors"
          >
            {t("nav.docs")}
          </a>
          <LanguageSelector />
        </div>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/generate" element={<GeneratePage />} />
          <Route path="/build" element={<ConfigBuilderPage />} />
        </Routes>
      </BrowserRouter>
    </ConfigProvider>
  );
}

export default App;
