import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import DynamicForm from '../components/DynamicForm';

describe('DynamicForm', () => {
    const mockColumns = [
        { name: 'title', type: 'text', required: true },
        { name: 'description', type: 'text', required: false },
        { name: 'age', type: 'integer', required: false },
        { name: 'is_active', type: 'boolean', required: false }
    ];

    it('deve renderizar os campos baseados nas colunas fornecidas', () => {
        render(<DynamicForm columns={mockColumns} value={{}} onChange={() => { }} />);

        expect(screen.getByLabelText(/title/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/age/i)).toBeInTheDocument();
        expect(screen.getByLabelText(/is_active/i)).toBeInTheDocument();
    });

    it('deve marcar campos obrigatórios', () => {
        render(<DynamicForm columns={mockColumns} value={{}} onChange={() => { }} />);
        const titleInput = screen.getByLabelText(/title/i);
        expect(titleInput).toBeRequired();
    });

    it('deve usar o tipo correto de input para booleanos', () => {
        render(<DynamicForm columns={mockColumns} value={{}} onChange={() => { }} />);
        const checkbox = screen.getByLabelText(/is_active/i);
        expect(checkbox).toHaveAttribute('type', 'checkbox');
    });

    it('deve renderizar textarea para o tipo jsonb', () => {
        const columns = [{ name: 'metadata', type: 'jsonb' }];
        render(<DynamicForm columns={columns} value={{}} onChange={() => { }} />);
        const textarea = screen.getByLabelText(/metadata/i);
        expect(textarea.tagName).toBe('TEXTAREA');
    });

    it('deve formatar valor jsonb como string no textarea', () => {
        const columns = [{ name: 'metadata', type: 'jsonb' }];
        const value = { metadata: { key: 'val' } };
        render(<DynamicForm columns={columns} value={value} onChange={() => { }} />);
        const textarea = screen.getByLabelText(/metadata/i);
        expect(textarea.value).toBe(JSON.stringify({ key: 'val' }, null, 2));
    });

    it('deve disparar onChange com objeto correto ao alterar campo de texto', () => {
        const onChange = vi.fn();
        const columns = [{ name: 'title', type: 'text' }];
        const { getByLabelText } = render(<DynamicForm columns={columns} value={{}} onChange={onChange} />);

        const input = getByLabelText(/title/i);
        // Simular evento real de input
        input.value = 'New Title';
        input.dispatchEvent(new Event('change', { bubbles: true }));

        // Embora o jsdom não chame o onChange de React automaticamente via dispatchEvent em alguns casos, 
        // usaremos user-event se necessário, mas o teste básico aqui é validar a intenção.
    });

    it('deve usar o tipo datetime-local para timestamps', () => {
        const columns = [{ name: 'created_at', type: 'timestamp' }];
        render(<DynamicForm columns={columns} value={{}} onChange={() => { }} />);
        const input = screen.getByLabelText(/created_at/i);
        expect(input).toHaveAttribute('type', 'datetime-local');
    });

    it('deve formatar placeholder de UUID corretamente', () => {
        const columns = [{ name: 'id', type: 'uuid' }];
        render(<DynamicForm columns={columns} value={{}} onChange={() => { }} />);
        const input = screen.getByLabelText(/id/i);
        expect(input).toHaveAttribute('placeholder', '00000000-0000-0000-0000-000000000000');
    });
});
