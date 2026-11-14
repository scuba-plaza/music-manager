const generatedTypes = '/src/api/types.gen.ts';

export default {
  meta: {
    type: 'problem',
    schema: [],
    messages: { comment: 'Comments are not allowed: let names and tests carry the meaning.' },
  },
  create(context) {
    const file = '/' + context.filename.replaceAll('\\', '/');
    if (file.endsWith(generatedTypes)) {
      return {};
    }
    return {
      Program() {
        for (const comment of context.sourceCode.getAllComments()) {
          context.report({ loc: comment.loc, messageId: 'comment' });
        }
      },
    };
  },
};
