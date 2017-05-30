from setuptools import setup


def read_file(path):
    with open(path) as file:
        return file.read()


setup(
    name="magneticow",
    version="0.4.0",
    description="Lightweight web interface for magnetico.",
    long_description=read_file("README.rst"),
    url="http://magnetico.org",
    author="Mert Bora ALPER",
    author_email="bora@boramalper.org",
    license="GNU Affero General Public License v3 or later (AGPLv3+)",
    packages=["magneticow"],
    include_package_data=True,
    zip_safe=False,
    entry_points={
        "console_scripts": ["magneticow=magneticow.__main__:main"]
    },

    install_requires=[
        "appdirs >= 1.4.3",
        "flask >= 0.12.1",
        "gevent >= 1.2.1"
    ],

    classifiers=[
        "Development Status :: 2 - Pre-Alpha",
        "Environment :: Web Environment",
        "Intended Audience :: End Users/Desktop",
        "License :: OSI Approved :: GNU Affero General Public License v3 or later (AGPLv3+)",
        "Natural Language :: English",
        "Operating System :: POSIX :: Linux",
        "Programming Language :: Python :: 3 :: Only",
        "Programming Language :: Python :: Implementation :: CPython"
    ]
)
