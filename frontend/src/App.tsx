import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import HomePage from './pages/HomePage';
import PreviewPage from './pages/PreviewPage';
import TermsOfServicePage from './pages/TermsOfServicePage';
import PrivacyPolicyPage from './pages/PrivacyPolicyPage';

const App: React.FC = () => {
	return (
		<Router>
			<Routes>
				<Route path='/' element={<HomePage />} />
				<Route path='/f/:fileId' element={<PreviewPage />} />
				<Route path='/terms' element={<TermsOfServicePage />} />
				<Route path='/privacy' element={<PrivacyPolicyPage />} />
			</Routes>
		</Router>
	);
};

export default App;
