export interface Toast {
	id: number;
	message: string;
	kind: 'success' | 'error';
}

let toasts = $state<Toast[]>([]);
let nextId = 0;

export function getToasts(): Toast[] {
	return toasts;
}

function push(message: string, kind: Toast['kind']) {
	const id = nextId++;
	toasts = [...toasts, { id, message, kind }];
	setTimeout(() => dismiss(id), 4000);
}

export function toastSuccess(message: string): void {
	push(message, 'success');
}

export function toastError(message: string): void {
	push(message, 'error');
}

export function dismiss(id: number): void {
	toasts = toasts.filter((t) => t.id !== id);
}
