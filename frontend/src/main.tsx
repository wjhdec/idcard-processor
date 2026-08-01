// Copyright 2026 wjhdec
// SPDX-License-Identifier: Apache-2.0

import {StrictMode} from 'react'
import {createRoot} from 'react-dom/client'
import App from './App'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <StrictMode>
        <App/>
    </StrictMode>
)
