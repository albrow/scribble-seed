scribble-seed
-------------

Version: X.X.X

Seed project for [scribble](https://github.com/albrow/scribble), a tiny 
static blog generator written in go.

Installation
------------

Scribble-seed is tagged with versions to match each version of scribble 
itself. On a *nix machine, you can use the following command to install
the appropriate version of scribble-seed (works only for versions >= 
0.3.2):

``` bash
git clone -b `scribble version` --depth 1 https://github.com/albrow/scribble-seed.git
```

If you are using windows or some other platform, or if you are using an
older version of scribble, use `scribble version` to get the version
of scribble itself. Then clone the repo using the -b tag, like the command
above. The version should be formatted like `v0.3.2`, which might not
match the output of `scribble version` for older versions.

More Information
----------------

Check out the [scribble](https://github.com/albrow/scribble) repository 
for more information about how the seed project is structured and how
the compilation process works.