import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import HomePage from './pages/HomePage';
import PreviewPage from './pages/PreviewPage';

const App: React.FC = () => {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/f/:fileId" element={<PreviewPage />} />
      </Routes>
    </Router>
  );
};

export default App;