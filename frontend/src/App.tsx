import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import HomePage from './pages/HomePage';
import PreviewPage from './pages/PreviewPage';
import TermsOfServicePage from './pages/TermsOfServicePage';
import PrivacyPolicyPage from './pages/PrivacyPolicyPage';
import AdminPage from './pages/AdminPage';

const App: React.FC = () => {
	return (
		<Router>
			<Routes>
				<Route path='/' element={<HomePage />} />
				<Route path='/f/:fileId' element={<PreviewPage />} />
				<Route path='/terms' element={<TermsOfServicePage />} />
				<Route path='/privacy' element={<PrivacyPolicyPage />} />
				<Route path='/admin' element={<AdminPage />} />
			</Routes>
		</Router>
	);
};

export default App;
