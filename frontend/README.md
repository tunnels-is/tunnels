# Tunnels Frontend

This is the frontend application for the Tunnels project, built with React and Vite. It provides a modern, responsive interface for managing tunnels, devices, users, and network configurations.

## Tech Stack

-   **Framework**: [React](https://react.dev/) with [Vite](https://vitejs.dev/)
-   **Styling**: [Tailwind CSS](https://tailwindcss.com/)
-   **UI Components**: [Radix UI](https://www.radix-ui.com/) and [Shadcn/UI](https://ui.shadcn.com/)
-   **State Management**: [Jotai](https://jotai.org/)
-   **Data Fetching**: [TanStack Query (React Query)](https://tanstack.com/query/latest)
-   **Routing**: [React Router](https://reactrouter.com/)
-   **Icons**: [Lucide React](https://lucide.dev/)

## Project Structure

The source code is organized as follows:

```
src/
├── api/                  # API client functions
│   ├── account.js
│   ├── auth.js
│   ├── client.js
│   ├── tunnels.js
│   └── ...
├── assets/               # Static assets (fonts, images)
├── components/           # Reusable UI components
│   ├── ui/               # Radix/Shadcn UI primitives
│   ├── app-sidebar.jsx
│   ├── DataTable.jsx
│   ├── TunnelCard.jsx
│   └── ...
├── hooks/                # Custom React hooks
│   ├── useAuth.js
│   ├── useTunnels.js
│   └── ...
├── lib/                  # Utilities and constants
│   ├── helpers.js
│   └── utils.js
├── pages/                # Route components (Views)
│   ├── Login.jsx
│   ├── Tunnels.jsx
│   ├── Settings.jsx
│   └── ...
├── providers/            # React Context providers
│   └── QueryProvider.jsx
├── stores/               # Global state (Jotai atoms)
│   ├── configStore.js
│   ├── userStore.js
│   └── ...
├── App.jsx               # Main application component
```

## Key Features

-   **Authentication**: Secure login, password reset, and 2FA support.
-   **Tunnel Management**: Create, edit, delete, connect, and disconnect tunnels.
-   **Device Management**: Monitor and manage connected devices.
-   **DNS Configuration**: Manage DNS records and settings.
-   **User & Group Management**: Administer users and access groups.
-   **Real-time Stats**: View connection statistics and logs.

## Getting Started

### Prerequisites

-   Node.js (v18 or later recommended)
-   npm or pnpm

### Installation

1.  Clone the repository.
2.  Navigate to the frontend directory.
3.  Install dependencies:

```bash
npm install
# or
pnpm install
```

### Running Development Server
NOTE: For development purposes, you must disable the TLS at `https://localhost:7777` (accessible through `tunnels/cmd/main`).
To start the local development server:

```bash
npm run dev
# or
pnpm dev
```

The application will be available at `http://localhost:5173` (or the port shown in your terminal).

### Building for Production

To build the application for production:

```bash
npm run build
# or
pnpm build
```

The build artifacts will be generated in the `dist` directory.
