import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { BrowserRouter } from 'react-router-dom';
import App from './App';

describe('App Component', () => {
  it('renders without crashing', () => {
    // The App component might render the Navbar and Routes
    render(
      <BrowserRouter>
        <App />
      </BrowserRouter>
    );
    
    // We expect the OmniMarket X header/brand to be somewhere on the screen
    expect(screen.getByText(/OmniMarket X/i)).toBeInTheDocument();
  });
});
