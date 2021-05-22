# smithy
A Go-based tool to work with Smithy API Specifications.

This code is extracted from [SADL](https://github.com/boynton/sadl), and extended to handle more Smithy use cases. 

The tool  reads multiple files, assembles them, and then outputs the resulting model. The input files can be any mix of
Smithy IDL or Smithy AST files in JSON. The default output is the "unparsing" of the assembled model to IDL into a file
per namespace. Alternate generators may be specified (see usage line), notably "ast", which just dumps the model as JSON.

This work is an independent implementation of the [1.0 Smithy Specification](https://awslabs.github.io/smithy/1.0/spec/core/index.html).
For more information about Smithy, its specification, and its supported tooling, see https://awslabs.github.io/smithy/.
