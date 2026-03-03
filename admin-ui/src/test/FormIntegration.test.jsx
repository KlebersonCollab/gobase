import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import DynamicForm from '../components/DynamicForm';

// Mock de API para o RelationshipSelect interno
vi.mock('../api', () => ({
    req: vi.fn()
}));

import { req } from '../api';

describe('Integration: DynamicForm with Relationships', () => {
    const mockColumns = [
        { name: 'user_id', type: 'uuid', required: true }
    ];
    const mockRelations = [
        { column: 'user_id', references_table: 'users', references_column: 'id' }
    ];

    it('deve usar RelationshipSelect quando uma coluna possui relacionamento', async () => {
        req.mockResolvedValue([{ id: 'u1', name: 'John Doe' }]);

        render(<DynamicForm
            columns={mockColumns}
            relations={mockRelations}
            value={{}}
            onChange={() => { }}
        />);

        // Deve aparecer o texto de loading ou a opção inicial do select de usuários
        expect(await screen.findByText(/Select users.../i)).toBeInTheDocument();
        expect(await screen.findByText('John Doe')).toBeInTheDocument();
    });
});
