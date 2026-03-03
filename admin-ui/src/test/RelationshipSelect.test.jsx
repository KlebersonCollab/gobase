import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import RelationshipSelect from '../components/RelationshipSelect';

// Mock do módulo de API
vi.mock('../api', () => ({
    req: vi.fn()
}));

import { req } from '../api';

describe('RelationshipSelect', () => {
    it('deve carregar dados da tabela relacionada ao montar', async () => {
        req.mockResolvedValueOnce([{ id: '1', name: 'User 1' }, { id: '2', name: 'User 2' }]);

        render(<RelationshipSelect
            table="users"
            value=""
            onChange={() => { }}
            labelField="name"
        />);

        await waitFor(() => {
            expect(req).toHaveBeenCalledWith('/api/collections/users');
        });

        expect(await screen.findByText('User 1')).toBeInTheDocument();
        expect(await screen.findByText('User 2')).toBeInTheDocument();
    });

    it('deve exibir estado de loading enquanto busca dados', () => {
        req.mockReturnValue(new Promise(() => { })); // Promise que nunca resolve
        render(<RelationshipSelect table="users" value="" onChange={() => { }} />);
        const select = screen.getByRole('combobox');
        expect(select).toBeDisabled();
    });

    it('deve lidar com erro na API graciosamente', async () => {
        req.mockRejectedValue(new Error('API Error'));
        render(<RelationshipSelect table="users" value="" onChange={() => { }} />);

        await waitFor(() => {
            const select = screen.getByRole('combobox');
            expect(select).not.toBeDisabled();
        });
    });

    it('deve usar fallback de ID se labelField não existir no objeto', async () => {
        req.mockResolvedValueOnce([{ id: 'uuid-1' }]); // Sem o campo 'name'
        render(<RelationshipSelect table="users" value="" onChange={() => { }} labelField="name" />);

        expect(await screen.findByText('uuid-1')).toBeInTheDocument();
    });
});
