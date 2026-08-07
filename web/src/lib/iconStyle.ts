// Every app gets a real-looking icon even when the catalog has no artwork
// for it: a deterministic (same app -> same look, every time) gradient
// tile in an Apple-app-icon style, instead of a flat single-color square
// with a letter. Picked from a curated palette so it always looks
// intentional, never a random/clashing color.
const GRADIENTS = [
	['#FF9966', '#FF5E62'],
	['#43CBFF', '#9708CC'],
	['#F857A6', '#FF5858'],
	['#4ADE80', '#16A34A'],
	['#38BDF8', '#2563EB'],
	['#FBBF24', '#F97316'],
	['#A78BFA', '#7C3AED'],
	['#F472B6', '#DB2777'],
	['#2DD4BF', '#0D9488'],
	['#818CF8', '#4F46E5'],
	['#FB7185', '#E11D48'],
	['#34D399', '#059669']
];

function hash(str: string): number {
	let h = 0;
	for (let i = 0; i < str.length; i++) {
		h = (h << 5) - h + str.charCodeAt(i);
		h |= 0;
	}
	return Math.abs(h);
}

export function iconGradient(seed: string): string {
	const [from, to] = GRADIENTS[hash(seed) % GRADIENTS.length];
	return `linear-gradient(135deg, ${from}, ${to})`;
}

export function iconInitial(name: string): string {
	return name.trim().charAt(0).toUpperCase() || '?';
}
