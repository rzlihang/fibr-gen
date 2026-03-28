import { ConfigProvider } from "antd";
import { BrowserRouter, Routes, Route } from "react-router";
import HomePage from "./pages/HomePage";
import GeneratePage from "./pages/GeneratePage";
import ConfigBuilderPage from "./pages/ConfigBuilderPage";

function App() {
  return (
    <ConfigProvider>
      <BrowserRouter>
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
