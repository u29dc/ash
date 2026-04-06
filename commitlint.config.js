export default {
	extends: ['@commitlint/config-conventional'],
	rules: {
		'type-enum': [2, 'always', ['feat', 'fix', 'refactor', 'docs', 'style', 'chore', 'test']],
		'scope-case': [2, 'always', 'lower-case'],
		'subject-case': [2, 'always', 'lower-case'],
		'header-max-length': [2, 'always', 100],
		'subject-full-stop': [2, 'never', '.'],
	},
};
